package runtime

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncProjectInvokesVenvAndSync(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(t.TempDir(), "uv")
	cache := t.TempDir()
	python := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]\nname = \"echo\"\nversion = \"0.1.0\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var got []*exec.Cmd
	u := UV{
		Bin:       bin,
		CacheDir:  cache,
		PythonDir: python,
		Run: func(cmd *exec.Cmd) error {
			got = append(got, snapshotCmd(cmd))
			return nil
		},
	}
	if err := u.SyncProject(dir); err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("runs = %d, want 3", len(got))
	}

	wantInstall := []string{bin, "python", "install", DefaultPython}
	if strings.Join(got[0].Args, " ") != strings.Join(wantInstall, " ") {
		t.Errorf("install = %q, want %q", got[0].Args, wantInstall)
	}

	wantVenv := []string{bin, "venv", "--python", DefaultPython, ".venv"}
	if strings.Join(got[1].Args, " ") != strings.Join(wantVenv, " ") {
		t.Errorf("venv = %q, want %q", got[1].Args, wantVenv)
	}
	wantSync := []string{bin, "sync", "--no-dev", "--no-editable"}
	if strings.Join(got[2].Args, " ") != strings.Join(wantSync, " ") {
		t.Errorf("sync = %q, want %q", got[2].Args, wantSync)
	}

	venv := filepath.Join(dir, ".venv")
	for i, cmd := range got[1:] {
		if cmd.Dir != dir {
			t.Errorf("cmd %d Dir = %q, want %s", i+1, cmd.Dir, dir)
		}
		env := envMap(cmd.Env)
		if env["UV_PROJECT_ENVIRONMENT"] != venv {
			t.Errorf("cmd %d UV_PROJECT_ENVIRONMENT = %q, want %q", i+1, env["UV_PROJECT_ENVIRONMENT"], venv)
		}
		if env["UV_PYTHON_PREFERENCE"] != "only-managed" {
			t.Errorf("cmd %d UV_PYTHON_PREFERENCE = %q", i+1, env["UV_PYTHON_PREFERENCE"])
		}
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

	var got []*exec.Cmd
	u := UV{
		Bin:       filepath.Join(t.TempDir(), "uv"),
		CacheDir:  t.TempDir(),
		PythonDir: t.TempDir(),
		Run: func(cmd *exec.Cmd) error {
			got = append(got, snapshotCmd(cmd))
			return nil
		},
	}
	if err := u.SyncProject(dir); err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("runs = %d, want 3", len(got))
	}
	want := []string{u.Bin, "sync", "--frozen", "--no-dev", "--no-editable"}
	if strings.Join(got[2].Args, " ") != strings.Join(want, " ") {
		t.Errorf("sync = %q, want %q", got[2].Args, want)
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
