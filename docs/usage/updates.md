---
title: Updates
description: Check the Golem version and install a newer release.
weight: 50
draft: false
---

**Settings → About** shows the running version. If a newer GitHub release exists, that page prints the latest tag and the install command.

To update, run the same installer as the first install:

```bash
curl -fsSL https://terracotta4u.com/golem/install.sh | sh
```

That replaces `~/.local/bin/golem` with the latest release (or set `GOLEM_VERSION` to pin a tag). Then restart `golem serve`.

`golem version` prints the binary version from the command line. Development builds do not report an update.
