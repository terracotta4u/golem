package main

import (
	"archive/zip"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/terracotta4u/golem/conf"
	"github.com/terracotta4u/golem/extension"
	"github.com/terracotta4u/golem/runtime"
)

func TestRunExtensionAddInstalls(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	src := t.TempDir()
	writePythonExt(t, src)
	stubEchoRuntime(t)

	stderr := captureStderr(t, func() {
		if err := run([]string{"extension", "add", src}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(stderr, "creating Python environment for echo") {
		t.Errorf("stderr = %q, want creating Python environment", stderr)
	}
	if !strings.Contains(stderr, "installed echo") {
		t.Errorf("stderr = %q, want installed echo", stderr)
	}

	dir, err := conf.ExtensionsDir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "echo", "pyproject.toml")); err != nil {
		t.Fatal(err)
	}

	cfg, _, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Extensions["echo"]; ok {
		t.Fatal("add should not write conf.json extensions")
	}
}

func TestRunExtensionAddRefusesDuplicate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	src := t.TempDir()
	writePythonExt(t, src)
	stubEchoRuntime(t)
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
	if _, _, err := conf.Load(); err != nil {
		t.Fatal(err)
	}
	writeConf(t, conf.Conf{
		Model: "openai/gpt-4o-mini",
		Extensions: map[string]conf.Extension{
			"echo": {Env: map[string]string{"ECHO_TOKEN": "secret"}},
		},
	})
	src := t.TempDir()
	writePythonExt(t, src)
	stubEchoRuntime(t)
	dir, err := conf.ExtensionsDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "echo"), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := run([]string{"extension", "add", "--force", src}); err != nil {
		t.Fatal(err)
	}

	cfg, _, err := conf.Load()
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
		"pyproject.toml": echoPyproject,
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

	stubEchoRuntime(t)
	if err := run([]string{"extension", "add", zipPath}); err != nil {
		t.Fatal(err)
	}

	dir, err := conf.ExtensionsDir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "echo", "pyproject.toml")); err != nil {
		t.Fatal(err)
	}
}

func TestRunExtensionList(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	src := t.TempDir()
	writePythonExt(t, src)
	stubEchoRuntime(t)
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
	if got != "echo  0.1.0  enabled" {
		t.Errorf("list = %q", got)
	}
}

func TestRunExtensionListMarksDisabled(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := conf.Load(); err != nil {
		t.Fatal(err)
	}
	dir, err := conf.ExtensionsDir()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "echo")
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "pyproject.toml"), []byte(echoPyproject), 0o600); err != nil {
		t.Fatal(err)
	}
	off := false
	writeConf(t, conf.Conf{
		Model: "openai/gpt-4o-mini",
		Extensions: map[string]conf.Extension{
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
	if got != "echo  0.1.0  disabled" {
		t.Errorf("list = %q", got)
	}
}

func TestRunExtensionRemove(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	src := t.TempDir()
	writePythonExt(t, src)
	stubEchoRuntime(t)
	if err := run([]string{"extension", "add", src}); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"extension", "remove", "echo"}); err != nil {
		t.Fatal(err)
	}

	dir, err := conf.ExtensionsDir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "echo")); !os.IsNotExist(err) {
		t.Fatal("install dir still present")
	}
	cfg, _, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Extensions["echo"]; ok {
		t.Fatal("echo still in conf")
	}
}

func writePythonExt(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(echoPyproject), 0o600); err != nil {
		t.Fatal(err)
	}
}

const echoPyproject = `[project]
name = "echo"
version = "0.1.0"

[project.scripts]
echo = "echo:main"
`

func stubEchoRuntime(t *testing.T) {
	t.Helper()
	restore := extension.StubRuntime(runtime.UV{
		Bin:       filepath.Join(t.TempDir(), "uv"),
		CacheDir:  t.TempDir(),
		PythonDir: t.TempDir(),
		Run: func(cmd *exec.Cmd) error {
			if cmd.Dir == "" {
				return nil
			}
			path := filepath.Join(cmd.Dir, ".venv", "bin", "echo")
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				return err
			}
			return os.WriteFile(path, []byte("#!/bin/sh\n"), 0o700)
		},
	})
	t.Cleanup(restore)
}

func TestRunExtensionRemoveMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	err := run([]string{"extension", "remove", "echo"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not installed") {
		t.Errorf("error = %v, want not installed", err)
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	fn()
	_ = w.Close()
	os.Stderr = old
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
