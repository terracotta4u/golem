package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSkill(t *testing.T) {
	got, err := Parse([]byte(`---
name: commit
description: Generate git commit messages from staged diffs. Use when the user asks to commit.
---

# Commit

1. Run git status
`))
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "commit" {
		t.Errorf("Name = %q, want commit", got.Name)
	}
	if !strings.Contains(got.Description, "commit messages") {
		t.Errorf("Description = %q", got.Description)
	}
	if !strings.Contains(got.Body, "# Commit") || !strings.Contains(got.Body, "git status") {
		t.Errorf("Body = %q", got.Body)
	}
}

func TestParseRequiresFrontmatter(t *testing.T) {
	_, err := Parse([]byte("# Commit\n\nNo frontmatter.\n"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseRequiresName(t *testing.T) {
	_, err := Parse([]byte(`---
description: Does something useful when asked.
---

# Body
`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseRequiresDescription(t *testing.T) {
	_, err := Parse([]byte(`---
name: commit
---

# Body
`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseRejectsInvalidName(t *testing.T) {
	_, err := Parse([]byte(`---
name: Commit Messages
description: Bad name.
---

# Body
`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadDirReadsSubdirectories(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "commit", `---
name: commit
description: Write commit messages.
---

Follow the commit format.
`)
	os.WriteFile(filepath.Join(root, "commit", "examples.md"), []byte("example"), 0o600)
	os.Mkdir(filepath.Join(root, "commit", "scripts"), 0o700)
	os.WriteFile(filepath.Join(root, "commit", "scripts", "msg.sh"), []byte("#!/bin/sh\n"), 0o700)
	os.WriteFile(filepath.Join(root, "notes.txt"), []byte("ignore me"), 0o600)

	got, err := LoadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("skills = %d, want 1: %+v", len(got), got)
	}
	if got[0].Name != "commit" || got[0].Dir != filepath.Join(root, "commit") {
		t.Errorf("skill = %+v", got[0])
	}
	if !strings.Contains(got[0].Body, "commit format") {
		t.Errorf("Body = %q", got[0].Body)
	}
}

func TestLoadDirSkipsMissingSkillMD(t *testing.T) {
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, "empty"), 0o700)
	writeSkill(t, root, "ok", `---
name: ok
description: A valid skill.
---

Body.
`)

	got, err := LoadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "ok" {
		t.Errorf("skills = %+v, want [ok]", got)
	}
}

func TestLoadDirSkipsInvalidSkill(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "bad", "# not a skill\n")
	writeSkill(t, root, "ok", `---
name: ok
description: A valid skill.
---

Body.
`)

	got, err := LoadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "ok" {
		t.Errorf("skills = %+v, want [ok]", got)
	}
}

func TestLoadDirDuplicateName(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "a", `---
name: commit
description: First.
---

A.
`)
	writeSkill(t, root, "b", `---
name: commit
description: Second.
---

B.
`)

	_, err := LoadDir(root)
	if err == nil {
		t.Fatal("expected error")
	}
}

func writeSkill(t *testing.T, root, dir, contents string) {
	t.Helper()
	path := filepath.Join(root, dir)
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}
