---
title: Memory
description: How Golem stores and retrieves lasting facts.
weight: 30
draft: false
---

After a turn, Golem asks the fast model to extract durable facts about you (preferences, standing context). One-off task details are skipped. Those facts are embedded and stored in `~/.golem/memory/memories.db`.

On later turns, Golem searches that store and may add matching memories to the system prompt. They are treated as context, not as instructions. They can be wrong or stale.

Settings live in `conf.json` under `memory`:

| Field | Default | Meaning |
|---|---|---|
| `embedding.provider` / `embedding.model` | `openrouter` / `openai/text-embedding-3-small` | Model used to embed memories |
| `budget_tokens` | `800` | Cap on how much retrieved memory is injected |
| `min_similarity` | `0.5` | Hits below this score are dropped |

The embedding provider must be a live extension that implements embeddings (OpenRouter does). If memory is missing from config, Golem does not retrieve or extract.
