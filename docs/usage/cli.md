---
title: CLI
description: Commands for serving Golem and managing extensions.
weight: 10
draft: false
---

```bash
golem serve
golem version
golem extension add SOURCE
golem extension list
golem extension remove NAME
```

## serve

Starts the HTTP server and every installed extension.

```bash
golem serve
golem serve --addr 127.0.0.1:9000
```

`--addr` defaults to `127.0.0.1:8743`. `--token` sets the API token; if you omit it, Golem generates one and prints it.

Extensions added from the CLI while the server is running are not picked up until you restart `golem serve`. Adding from **Settings → Extensions** starts them immediately.

## version

Prints the version, OS/arch, commit, and build date.

## extension

`add` installs from a directory, a zip file, or a GitHub URL:

```bash
golem extension add ./echo
golem extension add https://github.com/terracotta4u/golem-openrouter
golem extension add --ref v1.2.0 https://github.com/terracotta4u/golem-openrouter
golem extension add --force https://github.com/terracotta4u/golem-openrouter
```

`--ref` is a git ref for GitHub URLs (default is HEAD). `--force` replaces an existing install of the same name.

`list` prints name, version, and source. `remove` uninstalls by package name.
