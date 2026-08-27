package runtime

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsurePythonInvokesUV(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "uv")
	cache := filepath.Join(dir, "cache")
	python := filepath.Join(dir, "python")
	t.Setenv("UV_PYTHON", "/usr/bin/python3")
	t.Setenv("UV_PYTHON_PREFERENCE", "only-system")

	var got *exec.Cmd
	u := UV{
		Bin:       bin,
		CacheDir:  cache,
		PythonDir: python,
		Run: func(cmd *exec.Cmd) error {
			got = cmd
			return nil
		},
	}
	if err := u.EnsurePython(""); err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("Run not called")
	}
	if got.Path != bin {
		t.Errorf("Path = %q, want %q", got.Path, bin)
	}
	want := []string{bin, "python", "install", DefaultPython}
	if strings.Join(got.Args, " ") != strings.Join(want, " ") {
		t.Errorf("Args = %q, want %q", got.Args, want)
	}
	env := envMap(got.Env)
	if env["UV_CACHE_DIR"] != cache {
		t.Errorf("UV_CACHE_DIR = %q, want %q", env["UV_CACHE_DIR"], cache)
	}
	if env["UV_PYTHON_INSTALL_DIR"] != python {
		t.Errorf("UV_PYTHON_INSTALL_DIR = %q, want %q", env["UV_PYTHON_INSTALL_DIR"], python)
	}
	if env["UV_PYTHON_PREFERENCE"] != "only-managed" {
		t.Errorf("UV_PYTHON_PREFERENCE = %q, want only-managed", env["UV_PYTHON_PREFERENCE"])
	}
	if v, ok := env["UV_PYTHON"]; ok {
		t.Errorf("UV_PYTHON = %q, want stripped", v)
	}
	if _, err := os.Stat(python); err != nil {
		t.Errorf("python dir: %v", err)
	}
}

func TestEnsurePythonVersion(t *testing.T) {
	var got *exec.Cmd
	u := UV{
		Bin:       filepath.Join(t.TempDir(), "uv"),
		CacheDir:  t.TempDir(),
		PythonDir: t.TempDir(),
		Run: func(cmd *exec.Cmd) error {
			got = cmd
			return nil
		},
	}
	if err := u.EnsurePython("3.13"); err != nil {
		t.Fatal(err)
	}
	want := []string{u.Bin, "python", "install", "3.13"}
	if got == nil || strings.Join(got.Args, " ") != strings.Join(want, " ") {
		t.Errorf("Args = %v, want %v", got, want)
	}
}

func TestEnsurePythonRunError(t *testing.T) {
	u := UV{
		Bin:       filepath.Join(t.TempDir(), "uv"),
		CacheDir:  t.TempDir(),
		PythonDir: t.TempDir(),
		Run: func(*exec.Cmd) error {
			return errors.New("uv failed")
		},
	}
	err := u.EnsurePython(DefaultPython)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "uv failed") {
		t.Errorf("error = %v, want uv failed", err)
	}
}
