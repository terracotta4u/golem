---
title: Python SDK
description: Build providers, channels, and tools with golem-agent-sdk.
weight: 20
draft: false
---

Install from PyPI and import as `golem`:

```bash
pip install golem-agent-sdk
```

Golem injects `GOLEM_URL` and `GOLEM_TOKEN`, then runs `python -m golem` with the entrypoint from `[tool.golem]`. The HTTP contract is the [protocol](protocol.md).

## Providers

A provider is a model backend. Implement at least one of `chat`, `chat_structured`, or `embed`. The process binds a loopback server, registers, and heartbeats.

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

`id` is the name used in Golem conf (`default_model.provider` or `memory.embedding.provider`). An embeddings-only class can omit `chat`:

```python
class Embed(Provider):
    def embed(self, model, texts):
        return [[0.1] for _ in texts]
```

```toml
[tool.golem.provider]
id = "local-embed"
entrypoint = "golem_embed:Embed"
```

## Channels

A channel feeds messages into Golem. `run` is your loop. `client.send()` posts a turn and returns the assistant reply.

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

## Tools

A tool is a function Golem can call. The name is the function name, the description is the first docstring line, and parameters come from the annotations. Point `[tool.golem]` at a list, or at one function.

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

Once the extension is registered, Golem adds the tool at the start of each turn and posts the arguments object to `/v1/tools/weather`. The response is `{"result":"<text>"}`. A dict or list return value is JSON text inside that string. A raised exception is HTTP 500. The tool leaves the catalog when the extension expires.

Ship the package as described in [packaging](packaging.md).
