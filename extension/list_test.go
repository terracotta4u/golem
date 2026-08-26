package extension

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListReadsInstalled(t *testing.T) {
	root := t.TempDir()
	writeListed(t, root, "echo", `{"name":"echo","version":"0.1.0","kind":"channel","command":"./run"}`)
	writeListed(t, root, "telegram", `{"name":"telegram","version":"1.2.3","kind":"channel","command":"./bot"}`)
	if err := os.Mkdir(filepath.Join(root, "empty"), 0o700); err != nil {
		t.Fatal(err)
	}

	got, err := List(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("list = %+v, want 2", got)
	}
	if got[0].Name != "echo" || got[0].Version != "0.1.0" || got[0].Dir != filepath.Join(root, "echo") {
		t.Errorf("got[0] = %+v", got[0])
	}
	if got[1].Name != "telegram" || got[1].Version != "1.2.3" {
		t.Errorf("got[1] = %+v", got[1])
	}
}

func TestListEmptyDir(t *testing.T) {
	got, err := List(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("list = %+v, want empty", got)
	}
}

func TestListMissingDir(t *testing.T) {
	got, err := List(filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("list = %+v, want empty", got)
	}
}

func TestListNameMismatch(t *testing.T) {
	root := t.TempDir()
	writeListed(t, root, "echo", `{"name":"telegram","version":"0.1.0","kind":"channel","command":"./run"}`)
	_, err := List(root)
	if err == nil {
		t.Fatal("expected error")
	}
}

func writeListed(t *testing.T, root, name, manifest string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
}
