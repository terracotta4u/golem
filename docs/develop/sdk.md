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

Golem injects `GOLEM_URL` and `GOLEM_TOKEN`, then runs `python -m golem --name <package>` with `--provider`, `--channel`, and `--tools` taken from `[tool.golem]`. A package can declare any combination of those. Provider and channel entrypoints are classes, constructed with no arguments. The HTTP contract is the [protocol](protocol.md).

## Providers

A provider is a model backend. Override at least one of `chat`, `chat_structured`, or `embed`. Only overridden methods are advertised. The process binds a loopback server, registers, heartbeats every 10 seconds, and registers again if a heartbeat returns 404.

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

`id` is the name used in Golem config (`default_model.provider`, `fast_model.provider`, or `memory.embedding.provider`). An embeddings-only or structured-only class can omit `chat`:

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

A channel feeds messages into Golem. `run` is your loop. Golem sets `self.id` from the `id` in `[tool.golem.channel]` before calling it. `client.send(conversation_id, channel, text)` posts a turn and returns the assistant reply. An `error` event raises `GolemError`. If `run` raises, the process exits with status 1 and `golem serve` restarts it.

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

## Tools

A tool is a function Golem can call. The name is the function name, the description is the first non-empty docstring line, and parameters come from the annotations. Each parameter must be `str`, `int`, `float`, or `bool`. A parameter with a default is omitted from `required`. Golem calls the function with keywords, so an async function, a positional-only parameter, `*args`, or `**kwargs` is rejected when the extension starts. Golem also rejects the reserved names `read`, `write`, `edit`, and `shell`. Point `[tool.golem]` at a list of functions, or at one function. The attribute is used as-is; it is not called.

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

Once the extension is registered, Golem includes the tool at the start of each agent round and posts the arguments object to `/v1/tools/weather`. The response is `{"result":"<text>"}`. A dict or list return value is JSON text inside that string. Any other return value is sent as text. A raised exception is HTTP 500. The tool leaves the catalog when the extension expires.

Ship the package as described in [packaging](packaging.md).
