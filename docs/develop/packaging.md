---
title: Packaging
description: How to shape a Python package so Golem can install and run it.
weight: 10
draft: false
---

An extension is a Python project with a `pyproject.toml`. `golem extension add` copies it into `~/.golem/extensions/<name>/`, creates a venv, and runs `uv sync`. Then `golem serve` starts `.venv/bin/python -m golem` with the provider, channel, or tools from `[tool.golem]`.

If the project contains a `.python-version` file, Golem installs that Python and uses it for the venv. The first version line is the pin, for example `3.11`, `3.11.9`, or `cpython@3.12`. A `#` starts a comment. Without that file, Golem uses Python 3.12.

## pyproject.toml

Golem requires `name`, `version`, and at least one of a provider, a channel, or `tools`. A package may declare any combination of those. `name` is lowercase letters, digits, and hyphens (max 64 characters). That name is the install directory, the `--name` passed to `python -m golem`, and the `golem extension remove` argument.

```toml
[project]
name = "golem-echo"
version = "0.1.0"
description = "Example Golem extension"
requires-python = ">=3.11"
dependencies = [
    "golem-agent-sdk>=0.1.3",
]

[tool.golem.provider]
id = "echo"
entrypoint = "golem_echo:Echo"
```

`description` shows up in **Settings → Extensions**. `id` is the name used in Golem config. `entrypoint` is `module:Class`. Golem constructs that class with no arguments. Depend on [`golem-agent-sdk`](https://pypi.org/project/golem-agent-sdk/) unless you speak the [protocol](protocol.md) yourself.

A channel uses the same shape:

```toml
[tool.golem.channel]
id = "cli"
entrypoint = "golem_cli:CLI"
```

Tools point at a list of functions, or at one function. That object is the tool list:

```toml
[tool.golem]
tools = "golem_weather:tools"
```

## Install for development

From the extension directory, with Golem already installed:

```bash
golem extension add .
golem serve
```

Use `--force` to replace an existing install of the same name. A GitHub URL or zip works the same way; see [Extensions](../extensions/_index.md).
