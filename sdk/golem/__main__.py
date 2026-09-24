import importlib
import sys
import threading
from collections.abc import Callable
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
        TypeError: The imported object is not a ``Provider``, ``Channel``,
            or tool function.
    """
    if argv is None:
        argv = sys.argv[1:]
    name, provider, channel, tools = _args(argv)
    provider_impl: tuple[str, Provider] | None = None
    channel_impl: Channel | None = None
    tool_fns: list[Callable[..., Any]] | None = None
    if provider is not None:
        ident, entry = provider
        impl = _call(entry)
        if not isinstance(impl, Provider):
            raise TypeError(f"{entry} is not a Provider")
        provider_impl = (ident, impl)
    if channel is not None:
        ident, entry = channel
        impl = _call(entry)
        if not isinstance(impl, Channel):
            raise TypeError(f"{entry} is not a Channel")
        impl.id = ident
        channel_impl = impl
    if tools is not None:
        tool_fns = _tools(_load(tools))
    Extension(
        name,
        heartbeat_interval=heartbeat_interval,
        provider=provider_impl,
        channel=channel_impl,
        tools=tool_fns,
    ).run(stop)


def _args(
    argv: list[str],
) -> tuple[str, tuple[str, str] | None, tuple[str, str] | None, str | None]:
    name = ""
    provider: tuple[str, str] | None = None
    channel: tuple[str, str] | None = None
    tools: str | None = None
    i = 0
    while i < len(argv):
        flag = argv[i]
        if flag not in ("--name", "--provider", "--channel", "--tools"):
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
        elif flag == "--channel":
            if channel is not None:
                raise ValueError("channel already set")
            channel = _spec(value, "channel")
        else:
            if tools is not None:
                raise ValueError("tools already set")
            tools = _entrypoint(value)
    if not name:
        raise ValueError("name is required")
    if provider is None and channel is None and tools is None:
        raise ValueError("pass --provider, --channel, or --tools")
    return name, provider, channel, tools


def _spec(value: str, kind: str) -> tuple[str, str]:
    ident, sep, entry = value.partition("=")
    ident = ident.strip()
    entry = entry.strip()
    if not sep or not ident or not entry:
        raise ValueError(f"invalid {kind} {value!r}")
    return ident, _entrypoint(entry)


def _entrypoint(value: str) -> str:
    value = value.strip()
    module, esep, attr = value.partition(":")
    if not esep or not module or not attr or ":" in attr:
        raise ValueError(f"invalid entrypoint {value!r}")
    return value


def _load(entrypoint: str) -> Any:
    module_name, _, attr = entrypoint.partition(":")
    module = importlib.import_module(module_name)
    try:
        obj = getattr(module, attr)
    except AttributeError as exc:
        raise ValueError(f"entrypoint {entrypoint} not found") from exc
    return obj


def _call(entrypoint: str) -> Any:
    return _load(entrypoint)()


def _tools(obj: Any) -> list[Callable[..., Any]]:
    fns = obj if isinstance(obj, list) else [obj]
    if not fns:
        raise ValueError("tools list is empty")
    for fn in fns:
        if not callable(fn):
            raise TypeError(f"{fn!r} is not a tool")
    return fns


if __name__ == "__main__":
    serve()
