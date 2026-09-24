import json
import threading
import urllib.error
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any

import pytest

from golem.__main__ import serve

_PROVIDER = '''
from golem import Message, Provider

class Echo(Provider):
    def chat(self, model, messages, tools=None):
        text = messages[-1].content if messages else ""
        return Message(role="assistant", content="echo:" + text)
'''

_TOOLS = '''
def weather(city: str) -> str:
    """Current conditions for a city."""
    return "sunny in " + city

def broken(city: str) -> str:
    """Always fails."""
    raise RuntimeError("no station")

tools = [weather, broken]
'''

_CHANNEL = '''
import threading
from golem import Channel

started = threading.Event()

class CLI(Channel):
    def run(self, client, stop):
        CLI.seen_id = self.id
        CLI.client_url = client.url
        started.set()
        stop.wait()
'''


class _Golem:
    def __init__(self) -> None:
        self.registers: list[dict[str, Any]] = []
        self.registered = threading.Event()
        self.token = "secret"
        self.url = ""

    def handle(self, handler: BaseHTTPRequestHandler) -> None:
        auth = handler.headers.get("Authorization")
        if auth != "Bearer " + self.token:
            _write_json(handler, 401, {"error": "unauthorized"})
            return
        if handler.command == "GET" and handler.path == "/v1/health":
            _write_json(handler, 200, {"ok": True})
            return
        if handler.command == "POST" and handler.path == "/v1/extensions/register":
            self.registers.append(_read_json(handler))
            self.registered.set()
            _write_json(handler, 200, {"ok": True})
            return
        if handler.command == "POST" and handler.path == "/v1/extensions/heartbeat":
            _write_json(handler, 200, {"ok": True})
            return
        _write_json(handler, 404, {"error": "not found"})


@pytest.fixture
def golem(monkeypatch: pytest.MonkeyPatch) -> Any:
    state = _Golem()

    class Handler(BaseHTTPRequestHandler):
        protocol_version = "HTTP/1.1"

        def log_message(self, format: str, *args: object) -> None:
            return

        def do_GET(self) -> None:
            state.handle(self)

        def do_POST(self) -> None:
            state.handle(self)

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    host, port = server.server_address[:2]
    state.url = f"http://{host}:{port}"
    monkeypatch.setenv("GOLEM_URL", state.url)
    monkeypatch.setenv("GOLEM_TOKEN", state.token)
    try:
        yield state
    finally:
        server.shutdown()
        thread.join(timeout=2)


def test_provider_registers_and_chats(
    tmp_path: Any, monkeypatch: pytest.MonkeyPatch, golem: _Golem
) -> None:
    (tmp_path / "golem_echo.py").write_text(_PROVIDER)
    monkeypatch.syspath_prepend(str(tmp_path))
    stop = threading.Event()
    thread = threading.Thread(
        target=serve,
        kwargs={
            "argv": ["--name", "golem-echo", "--provider", "echo=golem_echo:Echo"],
            "stop": stop,
            "heartbeat_interval": 0.05,
        },
        daemon=True,
    )
    thread.start()
    try:
        assert golem.registered.wait(timeout=3)
        reg = golem.registers[0]
        assert reg["name"] == "golem-echo"
        assert reg["capabilities"] == [
            {"kind": "provider", "id": "echo", "chat": True}
        ]
        status, body = _callback_post(
            reg["callback_url"],
            "/v1/chat",
            golem.token,
            {"model": "m", "messages": [{"role": "user", "content": "hi"}]},
        )
        assert status == 200
        assert body["content"] == "echo:hi"
    finally:
        stop.set()
        thread.join(timeout=2)


def test_channel_registers_and_runs(
    tmp_path: Any, monkeypatch: pytest.MonkeyPatch, golem: _Golem
) -> None:
    (tmp_path / "golem_cli.py").write_text(_CHANNEL)
    monkeypatch.syspath_prepend(str(tmp_path))
    stop = threading.Event()
    thread = threading.Thread(
        target=serve,
        kwargs={
            "argv": ["--name", "golem-cli", "--channel", "cli=golem_cli:CLI"],
            "stop": stop,
            "heartbeat_interval": 0.05,
        },
        daemon=True,
    )
    thread.start()
    try:
        assert golem.registered.wait(timeout=3)
        assert golem.registers[0]["capabilities"] == [{"kind": "channel", "id": "cli"}]
        import golem_cli

        assert golem_cli.started.wait(timeout=2)
        assert golem_cli.CLI.seen_id == "cli"
        assert golem_cli.CLI.client_url == golem.url
    finally:
        stop.set()
        thread.join(timeout=2)


def test_tools_register_and_answer(
    tmp_path: Any, monkeypatch: pytest.MonkeyPatch, golem: _Golem
) -> None:
    (tmp_path / "golem_weather.py").write_text(_TOOLS)
    monkeypatch.syspath_prepend(str(tmp_path))
    stop = threading.Event()
    thread = threading.Thread(
        target=serve,
        kwargs={
            "argv": ["--name", "golem-weather", "--tools", "golem_weather:tools"],
            "stop": stop,
            "heartbeat_interval": 0.05,
        },
        daemon=True,
    )
    thread.start()
    try:
        assert golem.registered.wait(timeout=3)
        caps = golem.registers[0]["capabilities"]
        assert [cap["name"] for cap in caps] == ["weather", "broken"]
        assert caps[0]["kind"] == "tool"
        assert caps[0]["description"] == "Current conditions for a city."
        status, body = _callback_post(
            golem.registers[0]["callback_url"],
            "/v1/tools/weather",
            golem.token,
            {"city": "Lisbon"},
        )
        assert status == 200
        assert body == {"result": "sunny in Lisbon"}
        status, body = _callback_post(
            golem.registers[0]["callback_url"],
            "/v1/tools/broken",
            golem.token,
            {"city": "Lisbon"},
        )
        assert status == 500
        assert "no station" in body["error"]["message"]
    finally:
        stop.set()
        thread.join(timeout=2)


@pytest.mark.parametrize(
    ("argv", "match"),
    [
        ([], "name is required"),
        (["--name", "golem-echo"], "pass --provider, --channel, or --tools"),
        (["--name", "golem-echo", "--provider", "echo"], "invalid provider"),
        (
            ["--name", "golem-echo", "--provider", "echo=Echo"],
            "invalid entrypoint",
        ),
        (
            [
                "--name",
                "golem-echo",
                "--provider",
                "echo=golem_echo:Echo",
                "--provider",
                "other=golem_echo:Echo",
            ],
            "provider already set",
        ),
        (["--bogus"], "unknown argument"),
    ],
)
def test_serve_rejects_bad_args(argv: list[str], match: str) -> None:
    with pytest.raises(ValueError, match=match):
        serve(argv)


def _callback_post(
    url: str, path: str, token: str, body: dict[str, Any]
) -> tuple[int, Any]:
    data = json.dumps(body).encode()
    req = urllib.request.Request(
        url.rstrip("/") + path,
        data=data,
        headers={
            "Authorization": "Bearer " + token,
            "Content-Type": "application/json",
        },
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=5) as resp:
            return resp.status, json.loads(resp.read())
    except urllib.error.HTTPError as exc:
        raw = exc.read()
        parsed = json.loads(raw) if raw else {}
        return exc.code, parsed


def _read_json(handler: BaseHTTPRequestHandler) -> dict[str, Any]:
    length = int(handler.headers.get("Content-Length") or 0)
    if length == 0:
        return {}
    return json.loads(handler.rfile.read(length))


def _write_json(
    handler: BaseHTTPRequestHandler, status: int, body: dict[str, Any]
) -> None:
    raw = json.dumps(body).encode()
    handler.send_response(status)
    handler.send_header("Content-Type", "application/json")
    handler.send_header("Content-Length", str(len(raw)))
    handler.end_headers()
    handler.wfile.write(raw)
