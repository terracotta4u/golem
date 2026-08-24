package agent

import (
	"strings"
	"testing"

	"github.com/terracotta4u/golem/skill"
)

func TestSystemPromptOmitsCatalogWhenEmpty(t *testing.T) {
	got := systemPrompt(nil)
	if strings.Contains(got, "Skills:") || strings.Contains(got, "skill tool") {
		t.Errorf("empty catalog should omit skills, got %q", got)
	}
	if got != basePrompt {
		t.Errorf("got %q, want base prompt", got)
	}
}

func TestSystemPromptListsSkills(t *testing.T) {
	got := systemPrompt([]skill.Skill{{
		Name:        "commit",
		Description: "Write commit messages.",
	}})
	if !strings.Contains(got, "- commit: Write commit messages.") {
		t.Errorf("missing catalog line in %q", got)
	}
	if !strings.Contains(got, "skill tool") {
		t.Errorf("missing skill tool instruction in %q", got)
	}
}
