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

func TestResolveCommandVenvScript(t *testing.T) {
	dir := t.TempDir()
	writePythonSrc(t, dir)
	python := venvScript(dir, "python")
	if err := os.MkdirAll(filepath.Dir(python), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(python, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	p, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	cmd, args, err := ResolveCommand(dir, p)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"-m", "golem", "--name", "echo", "--provider", "echo=echo:Echo"}
	if cmd != python || strings.Join(args, " ") != strings.Join(want, " ") {
		t.Errorf("command = %q %q, want %q %q", cmd, args, python, want)
	}
}

func TestResolveCommandMissingVenvScript(t *testing.T) {
	dir := t.TempDir()
	writePythonSrc(t, dir)
	p, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = ResolveCommand(dir, p)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "python interpreter") {
		t.Errorf("error = %v, want python interpreter", err)
	}
}
