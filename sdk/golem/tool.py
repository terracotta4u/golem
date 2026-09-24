import inspect
from collections.abc import Callable
from typing import Any

_JSON_TYPES = {
    str: "string",
    int: "integer",
    float: "number",
    bool: "boolean",
}


def schema(fn: Callable[..., Any]) -> dict[str, Any]:
    """Describe ``fn`` for the model.

    The tool name is the function name. The description is the first line of
    its docstring. Parameters come from the signature: annotations become
    JSON types, and parameters with defaults are optional.

    Args:
        fn: Tool implementation. Called with the argument object as keywords.

    Returns:
        ``name``, ``description``, and ``parameters`` (a JSON Schema object).
    """
    doc = inspect.getdoc(fn) or ""
    description = ""
    for line in doc.splitlines():
        description = line.strip()
        if description:
            break
    properties: dict[str, Any] = {}
    required: list[str] = []
    for name, param in inspect.signature(fn).parameters.items():
        if param.kind in (param.VAR_POSITIONAL, param.VAR_KEYWORD):
            continue
        kind = _JSON_TYPES.get(param.annotation, "string")
        properties[name] = {"type": kind}
        if param.default is inspect.Parameter.empty:
            required.append(name)
    parameters: dict[str, Any] = {"type": "object", "properties": properties}
    if required:
        parameters["required"] = required
    return {"name": fn.__name__, "description": description, "parameters": parameters}


def invoke(fn: Callable[..., Any], args: dict[str, Any]) -> str:
    """Call ``fn`` with ``args`` and return its text.

    Args:
        fn: Tool implementation.
        args: Arguments object from ``POST /v1/tools/{name}``.

    Returns:
        The string the function returned, or ``str`` of another return value.
    """
    result = fn(**args)
    if isinstance(result, str):
        return result
    return str(result)
