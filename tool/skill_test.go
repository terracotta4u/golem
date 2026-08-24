package tool

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/terracotta4u/golem/skill"
)

func TestSkillLoadsBodyDirAndFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("ignored"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "examples.md"), []byte("ex"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "scripts"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scripts", "msg.sh"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	got, err := NewSkill([]skill.Skill{{
		Name:        "commit",
		Description: "Write commit messages.",
		Body:        "Follow the commit format.",
		Dir:         dir,
	}}).Call(context.Background(), mustJSON(t, map[string]any{"name": "commit"}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, dir) {
		t.Errorf("missing dir %q in %q", dir, got)
	}
	if !strings.Contains(got, "Follow the commit format.") {
		t.Errorf("missing body in %q", got)
	}
	if !strings.Contains(got, "examples.md") {
		t.Errorf("missing examples.md in %q", got)
	}
	if !strings.Contains(got, "scripts/msg.sh") {
		t.Errorf("missing scripts/msg.sh in %q", got)
	}
	if strings.Contains(got, "SKILL.md") {
		t.Errorf("SKILL.md should not be listed: %q", got)
	}
}

func TestSkillPromptCatalog(t *testing.T) {
	got := NewSkill([]skill.Skill{{
		Name:        "commit",
		Description: "Write commit messages.",
	}}).ExtraPrompt()
	if !strings.Contains(got, "- commit: Write commit messages.") {
		t.Errorf("missing catalog line in %q", got)
	}
	if !strings.Contains(got, "skill tool") {
		t.Errorf("missing skill tool instruction in %q", got)
	}
	if NewSkill(nil).ExtraPrompt() != "" {
		t.Errorf("empty skills should not add prompt text")
	}
}

func TestSkillUnknownName(t *testing.T) {
	_, err := NewSkill(nil).Call(context.Background(), mustJSON(t, map[string]any{"name": "missing"}))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSkillRequiresName(t *testing.T) {
	_, err := NewSkill(nil).Call(context.Background(), mustJSON(t, map[string]any{}))
	if err == nil {
		t.Fatal("expected error")
	}
}
