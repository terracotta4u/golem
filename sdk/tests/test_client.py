import json
import threading
import urllib.parse
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

import pytest

from golem.client import Client, GolemError, TurnEvent


class _Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, format: str, *args: object) -> None:
        return

    def _read_json(self) -> dict:
        length = int(self.headers.get("Content-Length") or 0)
        if length == 0:
            return {}
        return json.loads(self.rfile.read(length))

    def _write_json(self, status: int, body: dict) -> None:
        raw = json.dumps(body).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def _write_sse(self, events: list[tuple[str, dict]]) -> None:
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.send_header("Cache-Control", "no-cache")
        self.end_headers()
        for name, data in events:
            payload = json.dumps(data)
            self.wfile.write(f"event: {name}\ndata: {payload}\n\n".encode())
            self.wfile.flush()

    def do_GET(self) -> None:
        auth = self.headers.get("Authorization")
        if auth != "Bearer secret":
            self._write_json(401, {"error": "unauthorized"})
            return
        if self.path == "/v1/health":
            self._write_json(200, {"ok": True})
            return
        if self.path == "/v1/conversations" or self.path.startswith(
            "/v1/conversations?"
        ):
            query = urllib.parse.parse_qs(urllib.parse.urlparse(self.path).query)
            channel = (query.get("channel") or [""])[0]
            items = [
                {
                    "id": "web-1",
                    "channel": "web",
                    "title": "Dinner plans",
                    "updated_at": "2026-01-02T03:04:05Z",
                },
                {
                    "id": "cli-1",
                    "channel": "cli",
                    "title": "Secret",
                    "updated_at": "2026-01-01T00:00:00Z",
                },
            ]
            if channel:
                items = [item for item in items if item["channel"] == channel]
            self._write_json(200, {"conversations": items})
            return
        if self.path == "/v1/conversations/web-1":
            self._write_json(
                200,
                {
                    "id": "web-1",
                    "channel": "web",
                    "title": "Dinner plans",
                    "updated_at": "2026-01-02T03:04:05Z",
                    "messages": [{"role": "user", "content": "hello"}],
                },
            )
            return
        if self.path.startswith("/v1/turns/"):
            turn_id = self.path.rsplit("/", 1)[-1]
            if turn_id == "turn-1":
                self._write_sse(
                    [
                        (
                            "log",
                            {
                                "line": "[read] {}",
                                "name": "read",
                                "args": "{}",
                                "result": "ok",
                            },
                        ),
                        ("done", {"text": "hello back"}),
                    ]
                )
                return
            if turn_id == "turn-err":
                self._write_sse([("error", {"error": "boom"})])
                return
            self._write_json(404, {"error": "turn not found"})
            return
        self._write_json(404, {"error": "not found"})

    def do_POST(self) -> None:
        auth = self.headers.get("Authorization")
        if auth != "Bearer secret":
            self._write_json(401, {"error": "unauthorized"})
            return
        if self.path == "/v1/conversations/chat-1/turns":
            req = self._read_json()
            if req.get("channel") != "telegram" or req.get("text") != "hello":
                self._write_json(400, {"error": "bad body"})
                return
            self._write_json(202, {"id": "turn-1"})
            return
        if self.path == "/v1/conversations/chat-err/turns":
            self._write_json(202, {"id": "turn-err"})
            return
        self._write_json(404, {"error": "not found"})


@pytest.fixture
def golem_url() -> str:
    server = ThreadingHTTPServer(("127.0.0.1", 0), _Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    host, port = server.server_address[:2]
    try:
        yield f"http://{host}:{port}"
    finally:
        server.shutdown()
        thread.join(timeout=2)


def test_post_turn_and_stream(golem_url: str) -> None:
    client = Client(golem_url, "secret")
    turn_id = client.post_turn("chat-1", "telegram", "hello")
    assert turn_id == "turn-1"
    events = list(client.stream_turn(turn_id))
    assert events[0] == TurnEvent(
        "log", {"line": "[read] {}", "name": "read", "args": "{}", "result": "ok"}
    )
    assert events[1] == TurnEvent("done", {"text": "hello back"})


def test_send_waits_for_done(golem_url: str) -> None:
    got = Client(golem_url, "secret").send("chat-1", "telegram", "hello")
    assert got == "hello back"


def test_send_turn_error(golem_url: str) -> None:
    with pytest.raises(GolemError, match="boom"):
        Client(golem_url, "secret").send("chat-err", "telegram", "hello")


def test_send_unauthorized(golem_url: str) -> None:
    with pytest.raises(GolemError, match="401"):
        Client(golem_url, "wrong").send("chat-1", "telegram", "hello")


def test_wait_ready(golem_url: str) -> None:
    Client(golem_url, "secret").wait_ready(timeout=2)


def test_list_conversations(golem_url: str) -> None:
    client = Client(golem_url, "secret")
    all_chats = client.list_conversations()
    assert [item["id"] for item in all_chats] == ["web-1", "cli-1"]
    web = client.list_conversations(channel="web")
    assert web == [all_chats[0]]


def test_conversation(golem_url: str) -> None:
    got = Client(golem_url, "secret").conversation("web-1")
    assert got["id"] == "web-1"
    assert got["messages"] == [{"role": "user", "content": "hello"}]


def test_conversation_not_found(golem_url: str) -> None:
    with pytest.raises(GolemError, match="404") as exc:
        Client(golem_url, "secret").conversation("missing")
    assert exc.value.status == 404


def test_from_env(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("GOLEM_URL", "http://127.0.0.1:9999/")
    monkeypatch.setenv("GOLEM_TOKEN", "secret")
    client = Client.from_env()
    assert client.url == "http://127.0.0.1:9999"
    assert client.token == "secret"
