package agent

import (
	"strings"
	"testing"

	"github.com/terracotta4u/golem/memory"
)

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
