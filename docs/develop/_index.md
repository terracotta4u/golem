---
title: Develop
description: Write extensions that Golem can run.
weight: 40
draft: false
---

Golem launches each extension as a subprocess. A process can be a **provider** (Golem calls you for chat or embeddings), a **channel** (you send turns into Golem), or both.

- [Packaging](packaging.md) — `pyproject.toml` and how Golem installs the package
- [Python SDK](sdk.md) — `golem-agent-sdk`, providers, and channels
- [Protocol](protocol.md) — HTTP register, heartbeat, chat, and embed
