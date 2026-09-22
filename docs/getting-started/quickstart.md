---
title: Quickstart
description: Start Golem, add a provider, and send a first message.
weight: 20
draft: false
---

## Start the server

```bash
golem serve
```

Golem listens on `http://127.0.0.1:8743` and prints a token. Open that URL in a browser. The token is for extensions talking to the local API, not for the web UI.

Leave this process running.

## Add a provider

Golem has no built-in model. Install a provider, then give it an API key. OpenRouter is a common first choice.

In another terminal:

```bash
golem extension add https://github.com/terracotta4u/golem-openrouter
export OPENROUTER_API_KEY=...
```

Stop the server (`Ctrl+C`) and run `golem serve` again so the new extension starts. The child process inherits `OPENROUTER_API_KEY` from your shell.

Default models in config already point at OpenRouter (`openai/gpt-4o-mini` for chat, `openai/text-embedding-3-small` for memory). You can change those later in Settings.

## Chat

Open `http://127.0.0.1:8743`, type a message, and send it. If the provider is missing or the key is unset, the extension will fail to start or to answer; check the terminal where `golem serve` is running.
