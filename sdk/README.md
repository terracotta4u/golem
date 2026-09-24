# Golem SDK

Python SDK for creating [Golem](https://github.com/terracotta4u/golem) extensions. This includes adding new channels, providers, and other long-running processes.

## Installation

This package is available on PyPI:

```bash
pip install golem-agent-sdk
```

## Usage

Import as `golem`. Golem injects `GOLEM_URL` and `GOLEM_TOKEN` into the process, then starts it with `python -m golem` and the entrypoint from `pyproject.toml`. The wire protocol is [docs/develop/protocol.md](../docs/develop/protocol.md).

### Provider Extensions

A provider is a model backend. Golem calls your process for chat, structured output, and/or embeddings. Implement at least one of those methods. The process binds a loopback callback, registers, and heartbeats.

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

`id` is the name used in Golem conf (`default_model.provider` or `memory.embedding.provider`). Override `chat_structured` or `embed` to advertise those routes. An embeddings-only class can omit `chat`:

```python
class Embed(Provider):
    def embed(self, model, texts):
        return [[0.1] for _ in texts]
```

Golem launches this as `python -m golem --name golem-embed --provider local-embed=golem_embed:Embed`.

### Channel Extensions

A channel feeds messages into Golem (Telegram, CLI, and so on). `run` is your loop. `client.send()` posts a turn and returns the assistant reply. The channel `id` is advertised to Golem.

```python
from golem import Channel


class CLI(Channel):
    def run(self, client, stop):
        while not stop.is_set():
            line = input("you: ").strip()
            if not line:
                continue
            print(client.send("local", self.id, line))
```

```toml
[tool.golem.channel]
id = "cli"
entrypoint = "golem_cli:CLI"
```

`post_turn` and `stream_turn` expose the same flow as SSE events (`log`, `done`, `error`).

### Tool Extensions

A tool is a function Golem can call. The name is the function name, the description is the first docstring line, and parameters come from the annotations. Point `[tool.golem]` at a list of those functions, or at one function:

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

Golem launches this as `python -m golem --name golem-weather --tools golem_weather:tools`. A request arrives as `POST /v1/tools/weather` with the arguments object, and the response is `{"result":"<text>"}`.

## Packaging Extensions

Ship the extension as a Python package. Golem takes the name and description from `pyproject.toml`, reads `[tool.golem]`, and runs `python -m golem` with that provider, channel, or tools list:

```toml
[project]
name = "golem-openrouter"
description = "OpenRouter provider for Golem"
dependencies = ["golem-agent-sdk"]

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

From the Golem repo root, `make test-sdk` runs ruff and pytest.

## Releasing

PyPI publishes happen from this repo on `sdk-v*` tags:

1. Bump the version in `pyproject.toml`.
2. Commit that change on `main`.
3. Tag the same version and push the tag:

```bash
git tag sdk-v0.1.2
git push origin sdk-v0.1.2
```

The tag must match `pyproject.toml` (`sdk-v0.1.2` for `version = "0.1.2"`). GitHub Actions builds from `sdk/` and uploads to PyPI.
