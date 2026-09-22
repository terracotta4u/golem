---
title: Install
description: Install Golem on macOS or Linux.
weight: 10
draft: false
---

Golem supports macOS and Linux on amd64 and arm64.

```bash
curl -fsSL https://terracotta4u.com/golem/install.sh | sh
```

The installer downloads the latest GitHub release, checks the checksum, and puts the `golem` binary in `~/.local/bin`. If that directory is not on your `PATH`, add it:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Confirm the install:

```bash
golem version
```

Then go to the [quickstart](quickstart.md).
