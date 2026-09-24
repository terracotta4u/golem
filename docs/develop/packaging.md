---
title: Packaging
description: How to shape a Python package so Golem can install and run it.
weight: 10
draft: false
---

An extension is a Python project with a `pyproject.toml`. `golem extension add` copies it into `~/.golem/extensions/<name>/`, creates a venv with Python 3.12, and runs `uv sync`. Then `golem serve` starts the console script.

## pyproject.toml

Golem requires `name`, `version`, and `[project.scripts]`. `name` is lowercase letters, digits, and hyphens (max 64 characters). That name is the install directory and the `golem extension remove` argument.

If a script has the same name as the project, Golem runs that. If there is exactly one script, Golem runs that. Otherwise it errors.

```toml
[project]
name = "golem-echo"
version = "0.1.0"
description = "Example Golem extension"
requires-python = ">=3.11"
dependencies = [
    "golem-agent-sdk>=0.1.1",
]

[project.scripts]
golem-echo = "golem_echo:main"
```

`description` shows up in **Settings → Extensions**. Depend on [`golem-agent-sdk`](https://pypi.org/project/golem-agent-sdk/) unless you speak the [protocol](protocol.md) yourself.

## Install for development

From the extension directory, with Golem already installed:

```bash
golem extension add .
golem serve
```

Use `--force` to replace an existing install of the same name. A GitHub URL or zip works the same way; see [Extensions](../extensions/_index.md).
