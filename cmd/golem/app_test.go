package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/terracotta4u/golem/conf"
)

func TestLoadSkillsCreatesDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	skills, err := loadSkills()
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 0 {
		t.Errorf("skills = %+v, want empty", skills)
	}

	dir, err := conf.SkillsDir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatal(err)
	}
}

func TestLoadSkillsReadsSkillMD(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir, err := conf.SkillsDir()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "commit")
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte(`---
name: commit
description: Write commit messages.
---

Follow the commit format.
`), 0o600); err != nil {
		t.Fatal(err)
	}

	skills, err := loadSkills()
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 || skills[0].Name != "commit" {
		t.Errorf("skills = %+v, want [commit]", skills)
	}
}
