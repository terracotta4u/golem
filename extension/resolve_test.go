package extension

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVenvScriptUnix(t *testing.T) {
	got := venvScriptFor("darwin", "/ext", "echo")
	want := filepath.Join("/ext", ".venv", "bin", "echo")
	if got != want {
		t.Errorf("venvScript = %q, want %q", got, want)
	}
}

func TestVenvScriptWindows(t *testing.T) {
	got := venvScriptFor("windows", `C:\ext`, "echo")
	want := filepath.Join(`C:\ext`, ".venv", "Scripts", "echo.exe")
	if got != want {
		t.Errorf("venvScript = %q, want %q", got, want)
	}
}

func TestResolveCommandRequiresPyproject(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(`{"name":"echo","version":"0.1.0","kind":"channel","command":"echo"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = ResolveCommand(dir, m)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "pyproject.toml") {
		t.Errorf("error = %v, want pyproject.toml", err)
	}
}

func TestResolveCommandVenvScript(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(`{"name":"echo","version":"0.1.0","kind":"channel","command":"echo","args":["--poll"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]\nname = \"echo\"\nversion = \"0.1.0\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	script := venvScript(dir, "echo")
	if err := os.MkdirAll(filepath.Dir(script), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	cmd, args, err := ResolveCommand(dir, m)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != script {
		t.Errorf("command = %q, want %q", cmd, script)
	}
	if len(args) != 1 || args[0] != "--poll" {
		t.Errorf("args = %q, want [--poll]", args)
	}
}

func TestResolveCommandPythonFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(`{"name":"echo","version":"0.1.0","kind":"channel","command":"./bot.py"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]\nname = \"echo\"\nversion = \"0.1.0\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bot.py"), []byte("print('ok')\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	py := venvPython(dir)
	if err := os.MkdirAll(filepath.Dir(py), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(py, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	cmd, args, err := ResolveCommand(dir, m)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != py {
		t.Errorf("command = %q, want %q", cmd, py)
	}
	if len(args) != 1 || args[0] != filepath.Join(dir, "bot.py") {
		t.Errorf("args = %q, want bot.py", args)
	}
}

func TestResolveCommandMissingVenvScript(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(`{"name":"echo","version":"0.1.0","kind":"channel","command":"echo"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]\nname = \"echo\"\nversion = \"0.1.0\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = ResolveCommand(dir, m)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "echo") {
		t.Errorf("error = %v, want echo", err)
	}
}
