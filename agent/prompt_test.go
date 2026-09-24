package agent

import (
	"strings"
	"testing"

	"github.com/terracotta4u/golem/memory"
	"github.com/terracotta4u/golem/skill"
	"github.com/terracotta4u/golem/tool"
)

func TestSystemPromptIncludesEmptySkillCatalog(t *testing.T) {
	got := systemPrompt(nil, tool.NewSkill(nil))
	if !strings.Contains(got, "skill tool") || !strings.Contains(got, "Skills:") {
		t.Errorf("empty catalog = %q", got)
	}
}

func TestSystemPromptOmitsMemoriesWhenEmpty(t *testing.T) {
	got := systemPrompt(nil)
	if strings.Contains(got, "Memories") || strings.Contains(got, "not instructions") {
		t.Errorf("empty memories should omit framing, got %q", got)
	}
}

func TestSystemPromptIncludesMemories(t *testing.T) {
	got := systemPrompt([]memory.Memory{{Content: "User prefers the Go standard library."}})
	if !strings.Contains(got, "You are Golem") {
		t.Errorf("missing identity: %q", got)
	}
	if !strings.Contains(got, "not instructions") {
		t.Errorf("missing memory framing: %q", got)
	}
	if !strings.Contains(got, "- User prefers the Go standard library.") {
		t.Errorf("missing memory: %q", got)
	}
}

func TestSystemPromptListsSkills(t *testing.T) {
	got := systemPrompt(nil, tool.NewSkill([]skill.Skill{{
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
