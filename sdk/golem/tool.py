import inspect
import json
from collections.abc import Callable
from typing import Any, get_type_hints

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
    JSON types, including annotations postponed by
    ``from __future__ import annotations``. Parameters with defaults are
    optional. Only ``str``, ``int``, ``float``, and ``bool`` are accepted.
    Async functions, positional-only parameters, ``*args``, and ``**kwargs``
    are rejected. Golem calls the function with the argument object as keywords.

    Args:
        fn: Tool implementation.

    Returns:
        ``name``, ``description``, and ``parameters`` (a JSON Schema object).

    Raises:
        ValueError: The function cannot be called from an arguments object.
    """
    if inspect.iscoroutinefunction(fn) or inspect.isasyncgenfunction(fn):
        raise ValueError(f"{fn.__name__}: async functions are not supported")
    doc = inspect.getdoc(fn) or ""
    description = ""
    for line in doc.splitlines():
        description = line.strip()
        if description:
            break
    hints = _hints(fn)
    properties: dict[str, Any] = {}
    required: list[str] = []
    for name, param in inspect.signature(fn).parameters.items():
        _reject_kind(fn, name, param)
        annotation = hints.get(name, param.annotation)
        kind = _JSON_TYPES.get(annotation)
        if kind is None:
            if annotation is inspect.Parameter.empty:
                raise ValueError(
                    f"{fn.__name__}: parameter {name!r} needs an annotation "
                    "of str, int, float, or bool"
                )
            raise ValueError(
                f"{fn.__name__}: parameter {name!r} has unsupported type "
                f"{_type_name(annotation)}"
            )
        properties[name] = {"type": kind}
        if param.default is inspect.Parameter.empty:
            required.append(name)
    parameters: dict[str, Any] = {"type": "object", "properties": properties}
    if required:
        parameters["required"] = required
    return {"name": fn.__name__, "description": description, "parameters": parameters}


def _reject_kind(fn: Callable[..., Any], name: str, param: inspect.Parameter) -> None:
    if param.kind is inspect.Parameter.VAR_POSITIONAL:
        raise ValueError(f"{fn.__name__}: *{name} is not supported")
    if param.kind is inspect.Parameter.VAR_KEYWORD:
        raise ValueError(f"{fn.__name__}: **{name} is not supported")
    if param.kind is inspect.Parameter.POSITIONAL_ONLY:
        raise ValueError(
            f"{fn.__name__}: positional-only parameter {name!r} is not supported"
        )


def _hints(fn: Callable[..., Any]) -> dict[str, Any]:
    try:
        return get_type_hints(fn)
    except (NameError, TypeError, ValueError) as exc:
        raise ValueError(f"{fn.__name__}: could not resolve annotations") from exc


def _type_name(annotation: Any) -> str:
    if isinstance(annotation, str):
        return annotation
    name = getattr(annotation, "__name__", None)
    if isinstance(name, str):
        return name
    return str(annotation)


def invoke(fn: Callable[..., Any], args: dict[str, Any]) -> str:
    """Call ``fn`` with ``args`` and return its text.

    Args:
        fn: Tool implementation.
        args: Arguments object from ``POST /v1/tools/{name}``.

    Returns:
        The string the function returned. A dict or list is JSON text.
        Anything else is ``str`` of the return value.
    """
    result = fn(**args)
    if isinstance(result, str):
        return result
    if isinstance(result, (dict, list)):
        return json.dumps(result, ensure_ascii=False)
    return str(result)
