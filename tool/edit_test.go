package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditUniqueMatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(path, []byte("aaa foo bbb"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := NewEdit().Call(context.Background(), mustJSON(t, map[string]any{
		"path":       path,
		"old_string": "foo",
		"new_string": "bar",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, path) || !strings.Contains(got, "1 replacement") {
		t.Errorf("result = %q", got)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "aaa bar bbb" {
		t.Errorf("file = %q, want %q", data, "aaa bar bbb")
	}
}

func TestEditReplaceAll(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(path, []byte("foo and foo"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := NewEdit().Call(context.Background(), mustJSON(t, map[string]any{
		"path":        path,
		"old_string":  "foo",
		"new_string":  "bar",
		"replace_all": true,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "2 replacement") {
		t.Errorf("result = %q", got)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "bar and bar" {
		t.Errorf("file = %q, want %q", data, "bar and bar")
	}
}

func TestEditMissingString(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(path, []byte("aaa foo bbb"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := NewEdit().Call(context.Background(), mustJSON(t, map[string]any{
		"path":       path,
		"old_string": "zzz",
		"new_string": "bar",
	}))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "old_string not found") {
		t.Errorf("error = %q, want old_string not found", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "aaa foo bbb" {
		t.Errorf("file changed: %q", data)
	}
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
