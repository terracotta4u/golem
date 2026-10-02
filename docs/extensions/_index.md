---
title: Extensions
description: Install and run provider and channel extensions.
weight: 30
draft: false
---

An extension is a Python package Golem runs as a child process. There are two roles. One process can do both.

- **Provider** — a model backend. Golem calls it for chat, structured output, and/or embeddings.
- **Channel** — another way to send messages in (a chat app, a CLI, and so on).

Packages live in `~/.golem/extensions/`. Golem takes the name from `pyproject.toml` and runs `python -m golem` with the provider, channel, or tools declared there.

## Install

From the CLI (see [CLI](../usage/cli.md)):

```bash
golem extension add https://github.com/owner/repo
golem extension add ./echo
golem extension add ./echo.zip
```

Or in the web UI: **Settings → Extensions → Add extension** (GitHub URL or zip). Adding from the UI starts the process immediately. Adding from the CLI while `golem serve` is running does not; restart the server.

`golem extension list` and **Settings → Extensions** show what is installed. Remove on the extension page stops the process and uninstalls it. `golem extension remove NAME` uninstalls only when `golem serve` is not running. While serve is running, `remove` and `golem extension add --force` refuse; use the extensions page, or stop the server. A new `golem extension add` still installs and waits for the next serve to start it. If adding from the page saves the extension but fails to start it, the extension stays installed and the next `golem serve` starts it.

## Keys

Golem injects `GOLEM_URL` and `GOLEM_TOKEN`. API keys come from the environment of `golem serve`, or from `extensions.<name>.env` in `~/.golem/conf.json`. See [Configuration](../usage/configuration.md).
