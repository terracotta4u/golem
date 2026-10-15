package runtime

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncProjectInvokesSync(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(t.TempDir(), "uv")
	cache := t.TempDir()
	python := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]\nname = \"echo\"\nversion = \"0.1.0\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var got *exec.Cmd
	u := UV{
		Bin:       bin,
		CacheDir:  cache,
		PythonDir: python,
		Run: func(cmd *exec.Cmd) error {
			got = snapshotCmd(cmd)
			return nil
		},
	}
	if err := u.SyncProject(dir); err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("uv not called")
	}

	want := []string{bin, "sync", "--no-dev", "--no-editable", "--python", DefaultPython}
	if strings.Join(got.Args, " ") != strings.Join(want, " ") {
		t.Errorf("sync = %q, want %q", got.Args, want)
	}
	if got.Dir != dir {
		t.Errorf("Dir = %q, want %s", got.Dir, dir)
	}
	venv := filepath.Join(dir, ".venv")
	env := envMap(got.Env)
	if env["UV_PROJECT_ENVIRONMENT"] != venv {
		t.Errorf("UV_PROJECT_ENVIRONMENT = %q, want %q", env["UV_PROJECT_ENVIRONMENT"], venv)
	}
	if env["UV_PYTHON_PREFERENCE"] != "only-managed" {
		t.Errorf("UV_PYTHON_PREFERENCE = %q", env["UV_PYTHON_PREFERENCE"])
	}
}

func TestSyncProjectFrozenWhenLockfilePresent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]\nname = \"echo\"\nversion = \"0.1.0\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "uv.lock"), []byte("# lock\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var got *exec.Cmd
	u := UV{
		Bin:       filepath.Join(t.TempDir(), "uv"),
		CacheDir:  t.TempDir(),
		PythonDir: t.TempDir(),
		Run: func(cmd *exec.Cmd) error {
			got = snapshotCmd(cmd)
			return nil
		},
	}
	if err := u.SyncProject(dir); err != nil {
		t.Fatal(err)
	}
	want := []string{u.Bin, "sync", "--frozen", "--no-dev", "--no-editable", "--python", DefaultPython}
	if got == nil || strings.Join(got.Args, " ") != strings.Join(want, " ") {
		t.Fatalf("sync = %v, want %v", got, want)
	}
}

func TestSyncProjectUsesPythonVersionFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]\nname = \"echo\"\nversion = \"0.1.0\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".python-version"), []byte("# pin\n\n3.11\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var got *exec.Cmd
	u := UV{
		Bin:       filepath.Join(t.TempDir(), "uv"),
		CacheDir:  t.TempDir(),
		PythonDir: t.TempDir(),
		Run: func(cmd *exec.Cmd) error {
			got = snapshotCmd(cmd)
			return nil
		},
	}
	if err := u.SyncProject(dir); err != nil {
		t.Fatal(err)
	}
	want := []string{u.Bin, "sync", "--no-dev", "--no-editable"}
	if got == nil || strings.Join(got.Args, " ") != strings.Join(want, " ") {
		t.Fatalf("sync = %v, want %v", got, want)
	}
	if got.Dir != dir {
		t.Errorf("Dir = %q, want %s", got.Dir, dir)
	}
}

func TestSyncProjectFrozenUsesPythonVersionFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]\nname = \"echo\"\nversion = \"0.1.0\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "uv.lock"), []byte("# lock\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".python-version"), []byte("3.13.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var got *exec.Cmd
	u := UV{
		Bin:       filepath.Join(t.TempDir(), "uv"),
		CacheDir:  t.TempDir(),
		PythonDir: t.TempDir(),
		Run: func(cmd *exec.Cmd) error {
			got = snapshotCmd(cmd)
			return nil
		},
	}
	if err := u.SyncProject(dir); err != nil {
		t.Fatal(err)
	}
	want := []string{u.Bin, "sync", "--frozen", "--no-dev", "--no-editable"}
	if got == nil || strings.Join(got.Args, " ") != strings.Join(want, " ") || got.Dir != dir {
		t.Fatalf("sync = %v, want %v in %s", got, want, dir)
	}
}

func snapshotCmd(cmd *exec.Cmd) *exec.Cmd {
	return &exec.Cmd{
		Path: cmd.Path,
		Args: append([]string{}, cmd.Args...),
		Dir:  cmd.Dir,
		Env:  append([]string{}, cmd.Env...),
	}
}
