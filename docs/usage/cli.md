---
title: CLI
description: Commands for serving Golem and managing extensions.
weight: 10
draft: false
---

## serve

Starts the HTTP server and every installed extension.

```bash
golem serve
golem serve --addr 127.0.0.1:9000
golem serve --addr 192.168.1.20:8743
golem serve --addr 0.0.0.0:8743 --url http://192.168.1.20:8743
golem serve --url https://golem.example.com
```

`--addr` defaults to `127.0.0.1:8743`. `--token` sets the API token; if you omit it, Golem generates one and prints it. For IPv6 loopback, use `--addr '[::1]:8743'`.

With the default address, Golem listens on loopback and the web UI does not ask you to sign in. Open it as `127.0.0.1` or `localhost`. Browser requests that change state must come from the same origin (including the port), and the UI cannot be embedded in a frame.

To reach Golem from another machine, listen on that machine's address or on a wildcard. A specific address is enough on its own. A wildcard address such as `0.0.0.0` needs `--url` set to the origin you will open in the browser. `--url https://golem.example.com` with the default loopback address is for a reverse proxy on the same machine: the proxy terminates TLS and forwards requests with that host.

Either of those modes asks for the startup token on a sign-in page. The session cookie is `HttpOnly` and `SameSite=Strict`, and it is `Secure` when `--url` is `https`. Requests for any other host are rejected. The extension API still uses its bearer token, and extension callbacks stay on loopback.

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
