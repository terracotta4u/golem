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
