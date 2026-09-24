from golem.tool import schema


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
