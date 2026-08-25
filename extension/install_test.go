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
	writeInstallSrc(t, src, `{"name":"echo","kind":"channel","command":"./run","env":["ECHO_TOKEN"]}`)
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

func TestInstallRefusesExisting(t *testing.T) {
	src := t.TempDir()
	destRoot := t.TempDir()
	writeInstallSrc(t, src, `{"name":"echo","kind":"channel","command":"./run"}`)
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
	writeInstallSrc(t, src, `{"name":"echo","kind":"channel","command":"./run"}`)
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
	writeInstallSrc(t, src, `{"name":"ollama","kind":"provider","command":"./ollama"}`)
	_, err := Install(src, t.TempDir(), false)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Errorf("error = %v, want not supported", err)
	}
}

func writeInstallSrc(t *testing.T, dir, manifest string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
}
