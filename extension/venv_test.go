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
	script := venvScript(dir, "echo")
	if err := os.MkdirAll(filepath.Dir(script), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	p, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	cmd, err := ResolveCommand(dir, p)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != script {
		t.Errorf("command = %q, want %q", cmd, script)
	}
}

func TestResolveCommandMissingVenvScript(t *testing.T) {
	dir := t.TempDir()
	writePythonSrc(t, dir)
	p, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ResolveCommand(dir, p)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "echo") {
		t.Errorf("error = %v, want echo", err)
	}
}
