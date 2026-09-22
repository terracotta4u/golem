---
title: Configuration
description: Where Golem stores data and how to set models.
weight: 20
draft: false
---

Golem keeps its state in `~/.golem`. The first `golem serve` creates it.

| Path | Purpose |
|---|---|
| `~/.golem/etc/conf.json` | Models, memory settings, extension origins |
| `~/.golem/conversations/` | Chat history |
| `~/.golem/extensions/` | Installed extension packages |
| `~/.golem/memory/` | Memory database |
| `~/.golem/skills/` | Skill folders |
| `~/.golem/runtime/` | Bundled `uv` and Python |

## Models

Defaults in `conf.json`:

- **Default model** — chat and other heavier work (`openrouter` / `openai/gpt-4o-mini`)
- **Fast model** — lighter work such as memory extraction (`openrouter` / `openai/gpt-4o-mini`)

Change them in the web UI under **Settings → General**, or by editing `conf.json`. `provider` must match a live provider extension id (for OpenRouter, `openrouter`).

**Max tool rounds** caps how many tool loops a single turn can run. `0` means no cap.

## Extension keys

API keys are not stored in `conf.json` by default. Export them in the shell that runs `golem serve` so child processes inherit them:

```bash
export OPENROUTER_API_KEY=...
golem serve
```

You can also put env vars on an extension entry in `conf.json` under `extensions.<name>.env`.
