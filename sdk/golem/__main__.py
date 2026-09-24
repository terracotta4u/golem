import importlib
import sys
import threading
from typing import Any

from golem.channel import Channel
from golem.extension import Extension
from golem.provider import Provider


def serve(
    argv: list[str] | None = None,
    *,
    stop: threading.Event | None = None,
    heartbeat_interval: float = 10.0,
) -> None:
    """Import the named classes, register with Golem, and block.

    Args:
        argv: Arguments after ``python -m golem``. Defaults to ``sys.argv[1:]``.
        stop: Ends the process when set. Omit it to stop on SIGINT/SIGTERM.
        heartbeat_interval: Seconds between heartbeats.

    Raises:
        ValueError: An argument, id, or entrypoint is missing or repeated.
        TypeError: The imported object is not a ``Provider`` or ``Channel``.
    """
    if argv is None:
        argv = sys.argv[1:]
    name, provider, channel = _args(argv)
    ext = Extension.from_env(name, heartbeat_interval=heartbeat_interval)
    if provider is not None:
        ident, entry = provider
        impl = _call(entry)
        if not isinstance(impl, Provider):
            raise TypeError(f"{entry} is not a Provider")
        ext.provider(ident, impl)
    if channel is not None:
        ident, entry = channel
        impl = _call(entry)
        if not isinstance(impl, Channel):
            raise TypeError(f"{entry} is not a Channel")
        impl.id = ident
        ext.capability({"kind": "channel", "id": impl.id})
        ext.task(impl.run)
    ext.run(stop)


def _args(argv: list[str]) -> tuple[str, tuple[str, str] | None, tuple[str, str] | None]:
    name = ""
    provider: tuple[str, str] | None = None
    channel: tuple[str, str] | None = None
    i = 0
    while i < len(argv):
        flag = argv[i]
        if flag not in ("--name", "--provider", "--channel"):
            raise ValueError(f"unknown argument {flag}")
        if i + 1 >= len(argv):
            raise ValueError(f"{flag} requires a value")
        value = argv[i + 1]
        i += 2
        if flag == "--name":
            name = value.strip()
        elif flag == "--provider":
            if provider is not None:
                raise ValueError("provider already set")
            provider = _spec(value, "provider")
        else:
            if channel is not None:
                raise ValueError("channel already set")
            channel = _spec(value, "channel")
    if not name:
        raise ValueError("name is required")
    if provider is None and channel is None:
        raise ValueError("pass --provider or --channel")
    return name, provider, channel


def _spec(value: str, kind: str) -> tuple[str, str]:
    ident, sep, entry = value.partition("=")
    ident = ident.strip()
    entry = entry.strip()
    if not sep or not ident or not entry:
        raise ValueError(f"invalid {kind} {value!r}")
    module, esep, attr = entry.partition(":")
    if not esep or not module or not attr or ":" in attr:
        raise ValueError(f"invalid entrypoint {entry!r}")
    return ident, entry


def _call(entrypoint: str) -> Any:
    module_name, _, attr = entrypoint.partition(":")
    module = importlib.import_module(module_name)
    try:
        obj = getattr(module, attr)
    except AttributeError as exc:
        raise ValueError(f"entrypoint {entrypoint} not found") from exc
    return obj()


if __name__ == "__main__":
    serve()
