package main

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/terracotta4u/golem/config"
)

func TestRunExtensionAddInstallsAndScaffolds(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "golem.json"), []byte(`{"name":"echo","version":"0.1.0","kind":"channel","command":"./run","env":["ECHO_TOKEN"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "run"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := run([]string{"extension", "add", src}); err != nil {
		t.Fatal(err)
	}

	dir, err := config.ExtensionsDir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "echo", "run")); err != nil {
		t.Fatal(err)
	}

	cfg, _, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	ext, ok := cfg.Extensions["echo"]
	if !ok {
		t.Fatal("missing echo extension")
	}
	if v, ok := ext.Env["ECHO_TOKEN"]; !ok || v != "" {
		t.Errorf("ECHO_TOKEN = %q present=%v, want empty stub", v, ok)
	}
}

func TestRunExtensionAddRefusesDuplicate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "golem.json"), []byte(`{"name":"echo","version":"0.1.0","kind":"channel","command":"./run"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "run"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"extension", "add", src}); err != nil {
		t.Fatal(err)
	}
	err := run([]string{"extension", "add", src})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error = %v, want --force", err)
	}
}

func TestRunExtensionAddForceKeepsSecrets(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	writeConfig(t, config.Config{
		Model: "openai/gpt-4o-mini",
		Extensions: map[string]config.Extension{
			"echo": {Env: map[string]string{"ECHO_TOKEN": "secret"}},
		},
	})
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "golem.json"), []byte(`{"name":"echo","version":"0.1.0","kind":"channel","command":"./run","env":["ECHO_TOKEN"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "run"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	dir, err := config.ExtensionsDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "echo"), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := run([]string{"extension", "add", "--force", src}); err != nil {
		t.Fatal(err)
	}

	cfg, _, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Extensions["echo"].Env["ECHO_TOKEN"] != "secret" {
		t.Errorf("wiped secret: %+v", cfg.Extensions["echo"])
	}
}

func TestRunExtensionAddFromZip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	src := t.TempDir()
	zipPath := filepath.Join(src, "echo.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	for name, body := range map[string]string{
		"golem.json": `{"name":"echo","version":"0.1.0","kind":"channel","command":"./run","env":["ECHO_TOKEN"]}`,
		"run":        "#!/bin/sh\n",
	} {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if err := run([]string{"extension", "add", zipPath}); err != nil {
		t.Fatal(err)
	}

	dir, err := config.ExtensionsDir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "echo", "run")); err != nil {
		t.Fatal(err)
	}
}

func TestRunExtensionList(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "golem.json"), []byte(`{"name":"echo","version":"0.1.0","kind":"channel","command":"./run"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "run"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"extension", "add", src}); err != nil {
		t.Fatal(err)
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	err = run([]string{"extension", "list"})
	w.Close()
	os.Stdout = old
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(data))
	if got != "echo  0.1.0  channel  enabled" {
		t.Errorf("list = %q", got)
	}
}

func TestRunExtensionListMarksDisabled(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	dir, err := config.ExtensionsDir()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "echo")
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "golem.json"), []byte(`{"name":"echo","version":"0.1.0","kind":"channel","command":"./run"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	off := false
	writeConfig(t, config.Config{
		Model: "openai/gpt-4o-mini",
		Extensions: map[string]config.Extension{
			"echo": {Enabled: &off},
		},
	})

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	err = run([]string{"extension", "list"})
	w.Close()
	os.Stdout = old
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(data))
	if got != "echo  0.1.0  channel  disabled" {
		t.Errorf("list = %q", got)
	}
}
