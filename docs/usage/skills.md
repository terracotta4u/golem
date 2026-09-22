---
title: Skills
description: Add reusable instructions Golem can load with the skill tool.
weight: 40
draft: false
---

A skill is a folder under `~/.golem/skills/` with a `SKILL.md` file. Golem lists skills in the system prompt. When one applies, the model is told to load it with the `skill` tool before following it.

Restart `golem serve` after you add or change a skill.

## Layout

```
~/.golem/skills/commit/SKILL.md
```

The folder name is only for you. The skill **name** comes from frontmatter and must be unique.

```markdown
---
name: commit
description: Generate git commit messages from staged diffs. Use when the user asks to commit.
---

# Commit

1. Run git status
2. Run git diff --staged
```

`name` is lowercase letters, digits, and hyphens (max 64 characters). `description` is required; it is what the model sees in the catalog.

Other files in the same folder are listed when the skill loads, so you can keep scripts or extra notes next to `SKILL.md`.
