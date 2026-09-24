---
title: Packaging
description: How to shape a Python package so Golem can install and run it.
weight: 10
draft: false
---

An extension is a Python project with a `pyproject.toml`. `golem extension add` copies it into `~/.golem/extensions/<name>/`, creates a venv with Python 3.12, and runs `uv sync`. Then `golem serve` starts `.venv/bin/python -m golem` with the provider or channel from `[tool.golem]`.

## pyproject.toml

Golem requires `name`, `version`, and at least one of `[tool.golem.provider]` or `[tool.golem.channel]`. `name` is lowercase letters, digits, and hyphens (max 64 characters). That name is the install directory and the `golem extension remove` argument.

```toml
[project]
name = "golem-echo"
version = "0.1.0"
description = "Example Golem extension"
requires-python = ">=3.11"
dependencies = [
    "golem-agent-sdk>=0.1.1",
]

[tool.golem.provider]
id = "echo"
entrypoint = "golem_echo:Echo"
```

`description` shows up in **Settings → Extensions**. `id` is the name used in Golem conf. `entrypoint` is `module:Class`. Depend on [`golem-agent-sdk`](https://pypi.org/project/golem-agent-sdk/) unless you speak the [protocol](protocol.md) yourself.

A channel uses the same shape:

```toml
[tool.golem.channel]
id = "cli"
entrypoint = "golem_cli:CLI"
```

## Install for development

From the extension directory, with Golem already installed:

```bash
golem extension add .
golem serve
```

Use `--force` to replace an existing install of the same name. A GitHub URL or zip works the same way; see [Extensions](../extensions/_index.md).
