package tool

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteCreatesParentDirs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "file.txt")
	content := "hello"

	got, err := NewWrite().Call(context.Background(), mustJSON(t, map[string]any{
		"path":    path,
		"content": content,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, path) {
		t.Errorf("result = %q", got)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != content {
		t.Errorf("file = %q, want %q", data, content)
	}
}
