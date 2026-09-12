package agent

import (
	"strings"
	"testing"

	"github.com/terracotta4u/golem/skill"
	"github.com/terracotta4u/golem/tool"
)

func TestSystemPromptOmitsCatalogWhenEmpty(t *testing.T) {
	got := systemPrompt()
	if strings.Contains(got, "Skills:") || strings.Contains(got, "skill tool") {
		t.Errorf("empty catalog should omit skills, got %q", got)
	}
}

func TestSystemPromptListsSkills(t *testing.T) {
	got := systemPrompt(tool.NewSkill([]skill.Skill{{
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
	return t.TempDir()
}
