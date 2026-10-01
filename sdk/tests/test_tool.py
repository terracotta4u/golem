import importlib.util
from pathlib import Path

import pytest

from golem.tool import invoke, schema


def weather(city: str) -> str:
    """Current conditions for a city.

    Extra detail stays out of the description.
    """
    return city


def forecast(city: str, days: int = 1) -> str:
    """Daily forecast."""
    return city


def test_schema_from_two_functions() -> None:
    got = schema(weather)
    assert got == {
        "name": "weather",
        "description": "Current conditions for a city.",
        "parameters": {
            "type": "object",
            "properties": {"city": {"type": "string"}},
            "required": ["city"],
        },
    }
    got = schema(forecast)
    assert got["name"] == "forecast"
    assert got["description"] == "Daily forecast."
    assert got["parameters"] == {
        "type": "object",
        "properties": {
            "city": {"type": "string"},
            "days": {"type": "integer"},
        },
        "required": ["city"],
    }


def test_schema_resolves_postponed_annotations(tmp_path: Path) -> None:
    path = tmp_path / "postponed_tools.py"
    path.write_text(
        "from __future__ import annotations\n"
        "\n"
        "def forecast(city: str, days: int = 1) -> str:\n"
        '    """Daily forecast."""\n'
        "    return city\n"
    )
    spec = importlib.util.spec_from_file_location("postponed_tools_schema", path)
    assert spec is not None and spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    got = schema(module.forecast)
    assert got["parameters"]["properties"]["days"]["type"] == "integer"
    assert got["parameters"]["properties"]["city"]["type"] == "string"
    assert got["parameters"]["required"] == ["city"]


def test_schema_allows_scalars_and_keyword_only() -> None:
    def tune(*, enabled: bool, ratio: float = 0.5) -> str:
        """Tune a setting."""
        return "ok"

    got = schema(tune)
    assert got["parameters"] == {
        "type": "object",
        "properties": {
            "enabled": {"type": "boolean"},
            "ratio": {"type": "number"},
        },
        "required": ["enabled"],
    }


def test_schema_rejects_unsupported_signatures() -> None:
    def untyped(city):
        return city

    def listed(items: list[str]) -> str:
        return ""

    def variadic(city: str, *extra: str) -> str:
        return city

    def keywords(city: str, **extra: str) -> str:
        return city

    def positional(city: str, /) -> str:
        return city

    async def later(city: str) -> str:
        return city

    def nullable(city: str | None = None) -> str:
        return city or ""

    cases = [
        (untyped, "untyped: parameter 'city' needs an annotation"),
        (listed, "listed: parameter 'items' has unsupported type"),
        (variadic, r"variadic: \*extra is not supported"),
        (keywords, r"keywords: \*\*extra is not supported"),
        (positional, "positional: positional-only parameter 'city'"),
        (later, "later: async functions are not supported"),
        (nullable, "nullable: parameter 'city' has unsupported type"),
    ]
    for fn, match in cases:
        with pytest.raises(ValueError, match=match):
            schema(fn)


def test_schema_rejects_unresolved_annotations(tmp_path: Path) -> None:
    path = tmp_path / "unresolved_tools.py"
    path.write_text(
        "from __future__ import annotations\n"
        "\n"
        "def forecast(city: str, when: Missing) -> str:\n"
        '    """Daily forecast."""\n'
        "    return city\n"
    )
    spec = importlib.util.spec_from_file_location("unresolved_tools_schema", path)
    assert spec is not None and spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    with pytest.raises(ValueError, match="forecast: could not resolve annotations"):
        schema(module.forecast)


def test_invoke_returns_text() -> None:
    assert invoke(lambda: "sunny", {}) == "sunny"
    assert invoke(lambda: 3, {}) == "3"


def test_invoke_json_for_dict_and_list() -> None:
    assert invoke(lambda: {"city": "Lisbon", "temp": 72}, {}) == (
        '{"city": "Lisbon", "temp": 72}'
    )
    assert invoke(lambda: ["sunny", "warm"], {}) == '["sunny", "warm"]'
