import tomllib
from dataclasses import dataclass
from typing import Any


@dataclass(frozen=True)
class Entrypoint:
    """An object to import, written as ``module:attr``.

    Attributes:
        module: Import path, for example ``golem_echo``.
        attr: Attribute on that module, for example ``Echo``.
    """

    module: str
    attr: str

    def __str__(self) -> str:
        return f"{self.module}:{self.attr}"


@dataclass(frozen=True)
class Decl:
    """A provider or channel named in ``[tool.golem]``.

    Attributes:
        id: Capability id Golem records at install time.
        entrypoint: Class to instantiate with no arguments.
    """

    id: str
    entrypoint: Entrypoint


@dataclass(frozen=True)
class Manifest:
    """The parts of ``pyproject.toml`` the SDK reads.

    Attributes:
        name: ``[project].name``. Sent on register and heartbeat.
        provider: Set when ``[tool.golem.provider]`` is present.
        channel: Set when ``[tool.golem.channel]`` is present.
    """

    name: str
    provider: Decl | None = None
    channel: Decl | None = None


def parse(text: str) -> Manifest:
    """Parse extension manifest text.

    ``tools`` is ignored. A manifest with neither a provider nor a channel
    is an error.

    Args:
        text: Contents of ``pyproject.toml``.

    Returns:
        The project name and any provider or channel declaration.

    Raises:
        ValueError: The TOML is invalid, ``[project].name`` is missing, a
            declaration is incomplete, or neither a provider nor a channel
            is declared.
    """
    try:
        data = tomllib.loads(text)
    except tomllib.TOMLDecodeError as exc:
        raise ValueError(f"invalid pyproject.toml: {exc}") from exc

    name = _text(_table(data.get("project")).get("name"))
    if not name:
        raise ValueError("missing name")
    golem = _table(_table(data.get("tool")).get("golem"))
    provider = _decl(golem.get("provider"), "provider")
    channel = _decl(golem.get("channel"), "channel")
    if provider is None and channel is None:
        raise ValueError("declare a provider or channel in [tool.golem]")
    return Manifest(name=name, provider=provider, channel=channel)


def _table(value: Any) -> dict[str, Any]:
    if isinstance(value, dict):
        return value
    return {}


def _text(value: Any) -> str:
    if isinstance(value, str):
        return value.strip()
    return ""


def _decl(raw: Any, kind: str) -> Decl | None:
    if raw is None:
        return None
    table = _table(raw)
    if not table and not isinstance(raw, dict):
        raise ValueError(f"invalid [tool.golem.{kind}]")
    ident = _text(table.get("id"))
    if not ident:
        raise ValueError(f"{kind} id is required")
    entry = _text(table.get("entrypoint"))
    if not entry:
        raise ValueError(f"{kind} entrypoint is required")
    return Decl(id=ident, entrypoint=_entrypoint(entry))


def _entrypoint(value: str) -> Entrypoint:
    module, sep, attr = value.partition(":")
    if not sep or not module or not attr or ":" in attr:
        raise ValueError(f"invalid entrypoint {value!r}")
    return Entrypoint(module=module, attr=attr)
