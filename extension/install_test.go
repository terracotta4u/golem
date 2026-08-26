package extension

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallCopiesByManifestName(t *testing.T) {
	src := t.TempDir()
	destRoot := t.TempDir()
	writeInstallSrc(t, src, `{"name":"echo","version":"0.1.0","kind":"channel","command":"./run","env":["ECHO_TOKEN"]}`)
	if err := os.WriteFile(filepath.Join(src, "run"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	m, err := Install(src, destRoot, false)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "echo" {
		t.Errorf("manifest name = %q, want echo", m.Name)
	}

	dest := filepath.Join(destRoot, "echo")
	if _, err := os.Stat(filepath.Join(dest, FileName)); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dest, "run"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Errorf("run mode = %s, want executable", info.Mode())
	}
}

func TestInstallSkipsVenvAndJunk(t *testing.T) {
	src := t.TempDir()
	destRoot := t.TempDir()
	writeInstallSrc(t, src, `{"name":"echo","version":"0.1.0","kind":"channel","command":"./run"}`)
	if err := os.WriteFile(filepath.Join(src, "run"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "bot.py"), []byte("print('ok')\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	venvPython := filepath.Join(src, ".venv", "bin", "python")
	if err := os.MkdirAll(filepath.Dir(venvPython), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(venvPython, []byte("not-a-real-python\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	pycache := filepath.Join(src, "pkg", "__pycache__", "bot.cpython-312.pyc")
	if err := os.MkdirAll(filepath.Dir(pycache), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pycache, []byte("pyc"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "pkg", "bot.py"), []byte("x = 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	gitConfig := filepath.Join(src, ".git", "config")
	if err := os.MkdirAll(filepath.Dir(gitConfig), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gitConfig, []byte("[core]\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Install(src, destRoot, false); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(destRoot, "echo")
	if _, err := os.Stat(filepath.Join(dest, "run")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "bot.py")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "pkg", "bot.py")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, ".venv")); !os.IsNotExist(err) {
		t.Fatal("copied .venv")
	}
	if _, err := os.Stat(filepath.Join(dest, "pkg", "__pycache__")); !os.IsNotExist(err) {
		t.Fatal("copied __pycache__")
	}
	if _, err := os.Stat(filepath.Join(dest, ".git")); !os.IsNotExist(err) {
		t.Fatal("copied .git")
	}
}

func TestInstallRefusesExisting(t *testing.T) {
	src := t.TempDir()
	destRoot := t.TempDir()
	writeInstallSrc(t, src, `{"name":"echo","version":"0.1.0","kind":"channel","command":"./run"}`)
	if err := os.WriteFile(filepath.Join(src, "run"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(src, destRoot, false); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(src, destRoot, false); err == nil {
		t.Fatal("expected error")
	} else if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error = %v, want --force", err)
	}
}

func TestInstallForceReplaces(t *testing.T) {
	src := t.TempDir()
	destRoot := t.TempDir()
	writeInstallSrc(t, src, `{"name":"echo","version":"0.1.0","kind":"channel","command":"./run"}`)
	if err := os.WriteFile(filepath.Join(src, "run"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "old.txt"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(src, destRoot, false); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(src, "old.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "new.txt"), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(src, destRoot, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(destRoot, "echo", "new.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(destRoot, "echo", "old.txt")); !os.IsNotExist(err) {
		t.Fatal("force left old files in place")
	}
}

func TestInstallRejectsProvider(t *testing.T) {
	src := t.TempDir()
	writeInstallSrc(t, src, `{"name":"ollama","version":"0.1.0","kind":"provider","command":"./ollama"}`)
	_, err := Install(src, t.TempDir(), false)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Errorf("error = %v, want not supported", err)
	}
}

func TestRemoveDeletesInstall(t *testing.T) {
	src := t.TempDir()
	destRoot := t.TempDir()
	writeInstallSrc(t, src, `{"name":"echo","version":"0.1.0","kind":"channel","command":"./run"}`)
	if err := os.WriteFile(filepath.Join(src, "run"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(src, destRoot, false); err != nil {
		t.Fatal(err)
	}
	if err := Remove(destRoot, "echo"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(destRoot, "echo")); !os.IsNotExist(err) {
		t.Fatal("install dir still present")
	}
}

func TestRemoveMissing(t *testing.T) {
	err := Remove(t.TempDir(), "echo")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not installed") {
		t.Errorf("error = %v, want not installed", err)
	}
}

func TestRemoveRejectsInvalidName(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"", "../echo", "foo/bar", "Echo"} {
		if err := Remove(root, name); err == nil {
			t.Errorf("Remove(%q) succeeded, want error", name)
		}
	}
}

func writeInstallSrc(t *testing.T, dir, manifest string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
}
