---
title: Develop
description: Write extensions that Golem can run.
weight: 40
draft: false
---

Golem launches each extension as a subprocess. A process can advertise a **provider** (Golem calls you for chat, structured output, or embeddings), a **channel** (you send turns into Golem), **tools** (functions Golem calls during a turn), or any combination of those.

- [Packaging](packaging.md) — `pyproject.toml` and how Golem installs the package
- [Python SDK](sdk.md) — `golem-agent-sdk`, providers, channels, and tools
- [Protocol](protocol.md) — HTTP register, heartbeat, chat, structured output, embed, and tools
