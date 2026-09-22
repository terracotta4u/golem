---
title: Python SDK
description: Build providers and channels with golem-agent-sdk.
weight: 20
draft: false
---

Install from PyPI and import as `golem`:

```bash
pip install golem-agent-sdk
```

Golem injects `GOLEM_URL` and `GOLEM_TOKEN`. Prefer `Extension.from_env`. The HTTP contract is the [protocol](protocol.md).

## Minimal extension

Register, print a line, and stay alive until Golem stops the process:

```python
from golem import Extension


def hello(client, stop):
    print("hello world")
    stop.wait()


Extension.from_env("golem-echo").task(hello).run()
```

The name passed to `from_env` must match the installed package name.

## Providers

A provider is a model backend. Implement at least one of `chat`, `chat_structured`, or `embed`. `run()` binds a loopback server, registers, and heartbeats.

```python
from golem import Extension, Message, Provider


class Echo(Provider):
    def chat(self, model, messages, tools=None) -> Message:
        return Message(role="assistant", content=messages[-1].content)


Extension.from_env("golem-echo").provider("echo", Echo()).run()
```

The `id` passed to `provider()` is the name used in Golem conf (`default_model.provider` or `memory.embedding.provider`). An embeddings-only class can omit `chat`:

```python
class Embed(Provider):
    def embed(self, model, texts):
        return [[0.1] for _ in texts]


Extension.from_env("golem-embed").provider("local-embed", Embed()).run()
```

## Channels

A channel feeds messages into Golem. `task()` runs your loop. `client.send()` posts a turn and returns the assistant reply. `capability()` advertises the channel.

```python
from golem import Extension


def cli(client, stop):
    while not stop.is_set():
        line = input("you: ").strip()
        if not line:
            continue
        print(client.send("local", "cli", line))


(
    Extension.from_env("golem-cli")
    .capability({"kind": "channel", "id": "cli"})
    .task(cli)
    .run()
)
```

`post_turn` and `stream_turn` expose the same flow as SSE events (`log`, `done`, `error`).

Ship the package as described in [packaging](packaging.md).
