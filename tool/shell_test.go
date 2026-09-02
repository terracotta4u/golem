package tool

import (
	"context"
	"strings"
	"testing"
)

func TestShellReturnsStdout(t *testing.T) {
	got, err := NewShell().Call(context.Background(), mustJSON(t, map[string]any{
		"command": "echo hello",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "hello") {
		t.Errorf("got = %q, want hello", got)
	}
}

func TestShellTimeoutParameter(t *testing.T) {
	got, err := NewShell().Call(context.Background(), mustJSON(t, map[string]any{
		"command": "sleep 2",
		"timeout": 1,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "timed out after 1s") {
		t.Errorf("got = %q, want timed out after 1s", got)
	}
}

func TestShellMaxOutputParameter(t *testing.T) {
	got, err := NewShell().Call(context.Background(), mustJSON(t, map[string]any{
		"command":    "yes a | head -c 200",
		"max_output": 50,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "... (truncated)") {
		t.Errorf("got = %q, want truncated", got)
	}
	if len(got) > 50+len("\n... (truncated)") {
		t.Errorf("len(got) = %d, want at most %d", len(got), 50+len("\n... (truncated)"))
	}
}
