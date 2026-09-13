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
	got := systemPrompt(workspace(t))
	if strings.Contains(got, "Skills:") || strings.Contains(got, "skill tool") {
		t.Errorf("empty catalog should omit skills, got %q", got)
	}
}

func TestSystemPromptIncludesIdentityFiles(t *testing.T) {
	dir := workspace(t)
	if err := os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte("I am a test golem.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "USER.md"), []byte("The user is Nawaz.\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := systemPrompt(dir)
	soulPath := filepath.Join(dir, "SOUL.md")
	for _, want := range []string{
		soulPath,
		"I am a test golem.",
		"edit tool",
		"lasting",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("systemPrompt missing %q in %q", want, got)
		}
	}
	if strings.Contains(got, "USER.md") || strings.Contains(got, "The user is Nawaz.") {
		t.Errorf("systemPrompt still includes USER.md: %q", got)
	}
}

func TestSystemPromptListsSkills(t *testing.T) {
	got := systemPrompt(workspace(t), tool.NewSkill([]skill.Skill{{
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

func workspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"SOUL.md", "USER.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}
