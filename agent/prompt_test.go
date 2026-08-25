package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/terracotta4u/golem/skill"
	"github.com/terracotta4u/golem/tool"
)

func TestSystemPromptOmitsCatalogWhenEmpty(t *testing.T) {
	got := systemPrompt("")
	if strings.Contains(got, "Skills:") || strings.Contains(got, "skill tool") {
		t.Errorf("empty catalog should omit skills, got %q", got)
	}
	if strings.Contains(got, "SOUL.md") || strings.Contains(got, "USER.md") {
		t.Errorf("empty workspace should omit identity, got %q", got)
	}
	if got != basePrompt {
		t.Errorf("got %q, want base prompt", got)
	}
}

func TestSystemPromptIncludesIdentityFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte("I am a test golem.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "USER.md"), []byte("The user is Nawaz.\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := systemPrompt(dir)
	soulPath := filepath.Join(dir, "SOUL.md")
	userPath := filepath.Join(dir, "USER.md")
	for _, want := range []string{
		soulPath,
		userPath,
		"I am a test golem.",
		"The user is Nawaz.",
		"edit tool",
		"lasting",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("systemPrompt missing %q in %q", want, got)
		}
	}
}

func TestSystemPromptListsSkills(t *testing.T) {
	got := systemPrompt("", tool.NewSkill([]skill.Skill{{
		Name:        "commit",
		Description: "Write commit messages.",
	}}))
	if !strings.Contains(got, "- commit: Write commit messages.") {
		t.Errorf("missing catalog line in %q", got)
	}
	if !strings.Contains(got, "skill tool") {
		t.Errorf("missing skill tool instruction in %q", got)
	}
}
