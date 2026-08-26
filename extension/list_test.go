package extension

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestListReadsInstalled(t *testing.T) {
	root := t.TempDir()
	writeListed(t, root, "echo", "0.1.0")
	writeListed(t, root, "telegram", "1.2.3")
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
	dir := filepath.Join(root, "echo")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(projectTOML("telegram", "0.1.0")), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := List(root)
	if err == nil {
		t.Fatal("expected error")
	}
}

func writeListed(t *testing.T, root, name, version string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(projectTOML(name, version)), 0o600); err != nil {
		t.Fatal(err)
	}
}

func projectTOML(name, version string) string {
	return fmt.Sprintf("[project]\nname = %q\nversion = %q\n\n[project.scripts]\n%s = %q\n", name, version, name, name+":main")
}
