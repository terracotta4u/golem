from __future__ import annotations

import json
import signal
import sys
import threading
from collections.abc import Callable
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from types import FrameType
from typing import Any

from golem.channel import Channel
from golem.client import Client, GolemError
from golem.provider import JSONSchema, Message, Provider, ToolDef, UnsupportedFormat
from golem.tool import invoke
from golem.tool import schema as tool_schema

Task = Callable[[Client, threading.Event], None]


class Extension:
    """A Golem extension process: register, heartbeat, and optional callbacks.

    ``python -m golem`` constructs this. ``run`` binds a loopback HTTP server,
    registers with Golem, and blocks until SIGINT/SIGTERM or the optional
    ``stop`` event.

    Attributes:
        name: Extension name sent on register and heartbeat.
        client: Client used to talk to Golem.
        heartbeat_interval: Seconds between heartbeats.
    """

    def __init__(
        self,
        name: str,
        client: Client | str | None = None,
        token: str = "",
        *,
        heartbeat_interval: float = 10.0,
        provider: tuple[str, Provider] | None = None,
        channel: Channel | None = None,
        tools: list[Callable[..., Any]] | None = None,
    ) -> None:
        """Create an extension.

        Args:
            name: Extension name. Must match the installed package name.
            client: A ``Client``, a Golem base URL string, or ``None`` to
                use ``Client.from_env``.
            token: Bearer token when ``client`` is a URL string. Ignored
                otherwise.
            heartbeat_interval: Seconds between heartbeats. Keep this well
                under Golem's 30s registration TTL.
            provider: ``(id, implementation)``. Override ``chat``,
                ``chat_structured``, and/or ``embed`` to advertise those
                routes. At least one is required when this is set.
            channel: Loop advertised as ``{"kind": "channel", "id": channel.id}``
                and started after register.
            tools: Functions advertised as ``kind: tool`` and served at
                ``POST /v1/tools/{name}``.

        Raises:
            ValueError: ``name`` is empty, the provider id is empty, the
                provider implements no route, the channel id is empty, or
                two tools share a name.
        """
        name = name.strip()
        if not name:
            raise ValueError("name is required")
        self.name = name
        if isinstance(client, Client):
            self.client = client
        elif isinstance(client, str):
            self.client = Client(client, token)
        else:
            self.client = Client.from_env()
        self.heartbeat_interval = heartbeat_interval
        self._provider_id = ""
        self._provider: Provider | None = None
        self._tasks: list[Task] = []
        self._extra_caps: list[dict[str, Any]] = []
        self._tools: dict[str, Callable[..., Any]] = {}
        if provider is not None:
            self._set_provider(*provider)
        if channel is not None:
            self._set_channel(channel)
        if tools:
            self._set_tools(tools)

    def _set_provider(self, provider_id: str, impl: Provider) -> None:
        provider_id = provider_id.strip()
        if not provider_id:
            raise ValueError("provider id is required")
        if not any(
            _overrides(impl, method) for method in ("chat", "chat_structured", "embed")
        ):
            raise ValueError("provider must implement chat, chat_structured, or embed")
        self._provider_id = provider_id
        self._provider = impl

    def _set_channel(self, channel: Channel) -> None:
        channel_id = channel.id.strip()
        if not channel_id:
            raise ValueError("channel id is required")
        self._extra_caps.append({"kind": "channel", "id": channel_id})
        self._tasks.append(channel.run)

    def _set_tools(self, tools: list[Callable[..., Any]]) -> None:
        for fn in tools:
            spec = tool_schema(fn)
            name = str(spec["name"])
            if name in self._tools:
                raise ValueError(f"tool {name} already set")
            self._tools[name] = fn
            self._extra_caps.append({"kind": "tool", **spec})

    def run(self, stop: threading.Event | None = None) -> None:
        """Bind a loopback server, register, heartbeat, and block.

        Listens on ``127.0.0.1`` with an OS-assigned port, registers that
        URL with Golem, then waits until ``stop`` is set. If ``stop`` is
        omitted, SIGINT and SIGTERM set it (main thread only).

        Args:
            stop: Optional event to end the process. Created automatically
                when omitted.
        """
        own_stop = stop is None
        if stop is None:
            stop = threading.Event()
            if threading.current_thread() is threading.main_thread():

                def handle(_signum: int, _frame: FrameType | None) -> None:
                    stop.set()

                signal.signal(signal.SIGINT, handle)
                signal.signal(signal.SIGTERM, handle)

        server = ThreadingHTTPServer(("127.0.0.1", 0), _handler(self))
        callback = f"http://127.0.0.1:{server.server_address[1]}"
        serving = threading.Thread(target=server.serve_forever, daemon=True)
        serving.start()
        try:
            self._loop(callback, stop)
        finally:
            if own_stop:
                stop.set()
            server.shutdown()
            serving.join(timeout=2)
            server.server_close()

    def _loop(self, callback: str, stop: threading.Event) -> None:
        while not stop.is_set():
            try:
                self.client.wait_ready(timeout=5)
                self._register(callback)
                break
            except GolemError as exc:
                print(f"{self.name}: {exc}", file=sys.stderr, flush=True)
                if stop.wait(2):
                    return
        else:
            return

        threading.Thread(
            target=self._heartbeat_loop, args=(callback, stop), daemon=True
        ).start()
        for fn in self._tasks:
            threading.Thread(
                target=self._run_task, args=(fn, stop), daemon=True
            ).start()
        stop.wait()

    def _register(self, callback: str) -> None:
        self.client.register(self.name, callback, self._capabilities())

    def _heartbeat_loop(self, callback: str, stop: threading.Event) -> None:
        while not stop.wait(self.heartbeat_interval):
            try:
                self.client.heartbeat(self.name)
            except GolemError as exc:
                if exc.status == 404:
                    try:
                        self._register(callback)
                    except GolemError:
                        pass

    def _run_task(self, fn: Task, stop: threading.Event) -> None:
        try:
            fn(self.client, stop)
        except Exception as exc:  # noqa: BLE001
            print(f"{self.name} task: {exc}", file=sys.stderr, flush=True)

    def _capabilities(self) -> list[dict[str, Any]]:
        caps: list[dict[str, Any]] = []
        if self._provider is not None:
            cap: dict[str, Any] = {
                "kind": "provider",
                "id": self._provider_id,
            }
            if _overrides(self._provider, "chat"):
                cap["chat"] = True
            if _overrides(self._provider, "chat_structured"):
                cap["structured"] = True
            if _overrides(self._provider, "embed"):
                cap["embed"] = True
            caps.append(cap)
        caps.extend(self._extra_caps)
        return caps

    def _dispatch(self, path: str, body: dict[str, Any]) -> tuple[int, dict[str, Any]]:
        if path.startswith("/v1/tools/"):
            return self._invoke_tool(path.removeprefix("/v1/tools/").strip("/"), body)
        if self._provider is None:
            return 404, _error("not found")
        model = str(body.get("model") or "")
        try:
            if path == "/v1/chat":
                msg = self._provider.chat(model, _messages(body), _tools(body))
                return 200, msg.to_dict()
            if path == "/v1/chat/structured":
                raw = body.get("schema") if isinstance(body.get("schema"), dict) else {}
                data = self._provider.chat_structured(
                    model, _messages(body), JSONSchema.from_dict(raw)
                )
                return 200, {"data": data}
            if path == "/v1/embed":
                texts = body.get("texts") or []
                if not isinstance(texts, list):
                    return 400, _error("texts must be a list")
                vectors = self._provider.embed(model, [str(t) for t in texts])
                return 200, {"vectors": vectors}
        except UnsupportedFormat as exc:
            return 400, {
                "error": {
                    "code": "unsupported_format",
                    "message": str(exc) or "no json schema",
                }
            }
        except NotImplementedError:
            return 404, _error("not found")
        except Exception as exc:  # noqa: BLE001
            return 500, _error(str(exc))
        return 404, _error("not found")

    def _invoke_tool(self, name: str, body: dict[str, Any]) -> tuple[int, dict[str, Any]]:
        fn = self._tools.get(name)
        if fn is None:
            return 404, _error("not found")
        try:
            return 200, {"result": invoke(fn, body)}
        except Exception as exc:  # noqa: BLE001
            return 500, _error(str(exc))


def _overrides(provider: Provider, method: str) -> bool:
    return getattr(type(provider), method) is not getattr(Provider, method)


def _messages(body: dict[str, Any]) -> list[Message]:
    out: list[Message] = []
    for raw in body.get("messages") or []:
        if isinstance(raw, dict):
            out.append(Message.from_dict(raw))
    return out


def _tools(body: dict[str, Any]) -> list[ToolDef] | None:
    raw = body.get("tools") or []
    if not raw:
        return None
    return [ToolDef.from_dict(item) for item in raw if isinstance(item, dict)]


def _error(message: str) -> dict[str, Any]:
    return {"error": {"message": message}}


def _handler(ext: Extension) -> type[BaseHTTPRequestHandler]:
    class Handler(BaseHTTPRequestHandler):
        protocol_version = "HTTP/1.1"

        def log_message(self, format: str, *args: object) -> None:
            return

        def do_POST(self) -> None:
            if not self._authorized():
                self._write(401, _error("unauthorized"))
                return
            length = int(self.headers.get("Content-Length") or 0)
            raw = self.rfile.read(length) if length else b""
            if not raw:
                body: dict[str, Any] = {}
            else:
                try:
                    parsed = json.loads(raw)
                except json.JSONDecodeError:
                    self._write(400, _error("invalid json"))
                    return
                if not isinstance(parsed, dict):
                    self._write(400, _error("invalid json"))
                    return
                body = parsed
            status, payload = ext._dispatch(self.path, body)
            self._write(status, payload)

        def do_GET(self) -> None:
            if not self._authorized():
                self._write(401, _error("unauthorized"))
                return
            self._write(404, _error("not found"))

        def _authorized(self) -> bool:
            token = ext.client.token
            if not token:
                return True
            return self.headers.get("Authorization") == "Bearer " + token

        def _write(self, status: int, payload: dict[str, Any]) -> None:
            raw = json.dumps(payload).encode()
            self.send_response(status)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(raw)))
            self.end_headers()
            self.wfile.write(raw)

    return Handler
