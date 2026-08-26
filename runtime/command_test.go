package runtime

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandUsesAbsoluteUV(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "uv")
	cmd := Command(bin, filepath.Join(dir, "cache"), filepath.Join(dir, "python"), "python", "install", "3.12")
	if cmd.Path != bin {
		t.Errorf("Path = %q, want %q", cmd.Path, bin)
	}
	want := []string{bin, "python", "install", "3.12"}
	if strings.Join(cmd.Args, " ") != strings.Join(want, " ") {
		t.Errorf("Args = %q, want %q", cmd.Args, want)
	}
}

func TestCommandResolvesRelativeUV(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	cmd := Command("uv", "cache", "python")
	if !filepath.IsAbs(cmd.Path) {
		t.Errorf("Path = %q, want absolute", cmd.Path)
	}
	if cmd.Path != filepath.Join(dir, "uv") {
		t.Errorf("Path = %q, want %s", cmd.Path, filepath.Join(dir, "uv"))
	}
}

func TestCommandIsolatesUVEnv(t *testing.T) {
	dir := t.TempDir()
	cache := filepath.Join(dir, "cache")
	python := filepath.Join(dir, "python")
	t.Setenv("UV_CACHE_DIR", "/from-user/cache")
	t.Setenv("UV_PYTHON_INSTALL_DIR", "/from-user/python")
	t.Setenv("UV_PYTHON_PREFERENCE", "only-system")
	t.Setenv("UV_PYTHON", "/usr/bin/python3")
	t.Setenv("UV_SYSTEM_PYTHON", "1")
	t.Setenv("GOLEM_TEST_KEEP", "yes")

	cmd := Command(filepath.Join(dir, "uv"), cache, python)
	env := envMap(cmd.Env)

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
	if v, ok := env["UV_SYSTEM_PYTHON"]; ok {
		t.Errorf("UV_SYSTEM_PYTHON = %q, want stripped", v)
	}
	if env["GOLEM_TEST_KEEP"] != "yes" {
		t.Errorf("GOLEM_TEST_KEEP = %q, want parent env kept", env["GOLEM_TEST_KEEP"])
	}
}

func TestCommandDoesNotRun(t *testing.T) {
	// Command must not look up or execute uv; a missing binary is fine.
	cmd := Command(filepath.Join(t.TempDir(), "missing-uv"), t.TempDir(), t.TempDir(), "--version")
	if cmd.Process != nil {
		t.Fatal("Command started a process")
	}
}

func envMap(env []string) map[string]string {
	out := make(map[string]string)
	for _, kv := range env {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		out[k] = v
	}
	return out
}
