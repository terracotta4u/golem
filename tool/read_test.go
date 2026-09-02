package tool

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadOffsetLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\nfour\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := NewRead().Call(context.Background(), mustJSON(t, map[string]any{
		"path":   path,
		"offset": 2,
		"limit":  2,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "one") || strings.Contains(got, "four") {
		t.Errorf("got unexpected lines: %q", got)
	}
	if !strings.Contains(got, "2|two") || !strings.Contains(got, "3|three") {
		t.Errorf("got = %q, want numbered lines 2 and 3", got)
	}
}

func TestReadMaxOutputParameter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(path, []byte(strings.Repeat("a", 200)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := NewRead().Call(context.Background(), mustJSON(t, map[string]any{
		"path":       path,
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
