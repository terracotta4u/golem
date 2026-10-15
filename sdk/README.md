# Golem SDK

Python SDK for creating [Golem](https://github.com/terracotta4u/golem) extensions: providers, channels, and tools.

## Installation

This package is available on PyPI:

```bash
pip install golem-agent-sdk
```

## Usage

Import as `golem`. Golem injects `GOLEM_URL` and `GOLEM_TOKEN` into the process, then starts it with `python -m golem --name <package>` and the `--provider`, `--channel`, and `--tools` flags from `pyproject.toml`. A package can declare any combination. Provider and channel classes are constructed with no arguments. The wire protocol is [docs/develop/protocol.md](../docs/develop/protocol.md).

### Provider Extensions

A provider is a model backend. Golem calls your process for chat, structured output, and/or embeddings. Override at least one of those methods. Only overridden methods are advertised. The process binds a loopback callback, registers, heartbeats every 10 seconds, and registers again if a heartbeat returns 404.

```python
from golem import Message, Provider


class Echo(Provider):
    def chat(self, model, messages, tools=None) -> Message:
        return Message(role="assistant", content=messages[-1].content)
```

```toml
[tool.golem.provider]
id = "echo"
entrypoint = "golem_echo:Echo"
```

`id` is the name used in Golem config (`default_model.provider`, `fast_model.provider`, or `memory.embedding.provider`). Override `chat_structured` or `embed` to advertise those routes. An embeddings-only or structured-only class can omit `chat`:

```python
class Embed(Provider):
    def embed(self, model, texts):
        return [[0.1] for _ in texts]
```

Golem launches this as `python -m golem --name golem-embed --provider local-embed=golem_embed:Embed`.

### Channel Extensions

A channel feeds messages into Golem (Telegram, CLI, and so on). `run` is your loop. Golem sets `self.id` from the channel `id` in `pyproject.toml` before calling it, and advertises that id. `client.send(conversation_id, channel, text)` posts a turn and returns the assistant reply. An `error` event raises `GolemError`. If `run` raises, the process exits with status 1 and `golem serve` restarts it.

```python
from golem import Channel


class CLI(Channel):
    def run(self, client, stop):
        while not stop.is_set():
            line = input("you: ").strip()
            if not line:
                continue
            print(client.send("cli-session", self.id, line))
```

```toml
[tool.golem.channel]
id = "cli"
entrypoint = "golem_cli:CLI"
```

`post_turn` and `stream_turn` expose the same flow as SSE events (`log`, `done`, `error`).

### Tool Extensions

A tool is a function Golem can call. The name is the function name, the description is the first non-empty docstring line, and parameters come from the annotations. Each parameter must be `str`, `int`, `float`, or `bool`. A parameter with a default is omitted from `required`. Golem calls the function with keywords, so an async function, a positional-only parameter, `*args`, or `**kwargs` is rejected when the extension starts. Golem also rejects the reserved names `read`, `write`, `edit`, and `shell`. Point `[tool.golem]` at a list of those functions, or at one function. That attribute is the tool list:

```python
def weather(city: str) -> str:
    """Current conditions for a city."""
    return "sunny in " + city


tools = [weather]
```

```toml
[tool.golem]
tools = "golem_weather:tools"
```

Golem launches this as `python -m golem --name golem-weather --tools golem_weather:tools`. Once the extension is registered, Golem includes the tool at the start of each agent round and posts the arguments object to `/v1/tools/weather`. The response is `{"result":"<text>"}`. A dict or list return value is JSON text inside that string. Any other return value is sent as text. A raised exception is HTTP 500. The tool leaves the catalog when the extension expires.

## Packaging Extensions

Ship the extension as a Python package. Golem takes the name and description from `pyproject.toml`, reads `[tool.golem]`, and runs `python -m golem` with that provider, channel, or tools list. Provider and channel classes are constructed with no arguments:

```toml
[project]
name = "golem-openrouter"
description = "OpenRouter provider for Golem"
dependencies = ["golem-agent-sdk>=0.1.3"]

[tool.golem.provider]
id = "openrouter"
entrypoint = "golem_openrouter:OpenRouter"
```

## Development

Using uv is strongly recommended. From this directory:

```bash
uv sync --dev
uv run pytest
```

From the Golem repo root, `make test-sdk` runs ruff and pytest. `tests/test_smoke.py` starts the Go server in this repo and runs this SDK against it. That current pairing is the supported one.

## Releasing

PyPI publishes happen from this repo on `sdk-v*` tags:

1. Bump the version in `pyproject.toml`.
2. Commit that change on `main`.
3. Tag the same version and push the tag:

```bash
git tag sdk-v0.1.3
git push origin sdk-v0.1.3
```

The tag must match `pyproject.toml` (`sdk-v0.1.3` for `version = "0.1.3"`). GitHub Actions builds from `sdk/` and uploads to PyPI.
