package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/terracotta4u/golem/config"
	"github.com/terracotta4u/golem/extension"
	"github.com/terracotta4u/golem/runtime"
)

func TestServeStartsConfiguredExtension(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("OPENROUTER_API_KEY", "test")

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	if _, _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	extDir, err := config.ExtensionsDir()
	if err != nil {
		t.Fatal(err)
	}
	dir := writeProject(t, extDir, "echo")
	out := filepath.Join(t.TempDir(), "env")
	script := filepath.Join(dir, ".venv", "bin", "echo")
	if err := os.MkdirAll(filepath.Dir(script), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s %s' \"$GOLEM_URL\" \"$GOLEM_TOKEN\" > "+strconv.Quote(out)+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeConfig(t, config.Config{Listen: addr})

	app, err := loadApp()
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errc := make(chan error, 1)
	go func() { errc <- serve(ctx, app, addr) }()

	deadline := time.Now().Add(5 * time.Second)
	var got string
	for {
		data, err := os.ReadFile(out)
		if err == nil {
			got = strings.TrimSpace(string(data))
			parts := strings.SplitN(got, " ", 2)
			if len(parts) == 2 && parts[0] == "http://"+addr && parts[1] != "" {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("extension env = %q (%v), want GOLEM_URL=http://%s and a token", got, err, addr)
		}
		time.Sleep(20 * time.Millisecond)
	}

	cancel()
	if err := <-errc; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestExtensionListFillsFromInstall(t *testing.T) {
	root := t.TempDir()
	dir := writeProject(t, root, "echo")
	script := writeVenvEcho(t, dir)

	got, err := extensionList(config.Config{
		Extensions: map[string]config.Extension{
			"echo": {Env: map[string]string{"TOKEN": "x"}},
		},
	}, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("extensions = %d, want 1", len(got))
	}
	ext := got[0]
	if ext.Name != "echo" || ext.Command != script || ext.Dir != dir {
		t.Errorf("extension = %+v", ext)
	}
	if len(ext.Args) != 0 {
		t.Errorf("Args = %q, want empty", ext.Args)
	}
	if ext.Env["TOKEN"] != "x" {
		t.Errorf("Env = %v", ext.Env)
	}
}

func TestExtensionListStartsWithoutConfig(t *testing.T) {
	root := t.TempDir()
	dir := writeProject(t, root, "echo")
	script := writeVenvEcho(t, dir)

	got, err := extensionList(config.Config{}, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Command != script {
		t.Errorf("got = %+v, want %q", got, script)
	}
}

func TestExtensionListRepairsMissingVenv(t *testing.T) {
	root := t.TempDir()
	dir := writeProject(t, root, "echo")

	var runs int
	restore := extension.StubRuntime(runtime.UV{
		Bin:       filepath.Join(t.TempDir(), "uv"),
		CacheDir:  t.TempDir(),
		PythonDir: t.TempDir(),
		Run: func(cmd *exec.Cmd) error {
			runs++
			if cmd.Dir == "" {
				return nil
			}
			script := filepath.Join(cmd.Dir, ".venv", "bin", "echo")
			if err := os.MkdirAll(filepath.Dir(script), 0o700); err != nil {
				return err
			}
			return os.WriteFile(script, []byte("#!/bin/sh\n"), 0o700)
		},
	})
	t.Cleanup(restore)

	got, err := extensionList(config.Config{}, root)
	if err != nil {
		t.Fatal(err)
	}
	if runs == 0 {
		t.Fatal("did not repair missing .venv")
	}
	script := filepath.Join(dir, ".venv", "bin", "echo")
	if len(got) != 1 || got[0].Command != script {
		t.Errorf("Command = %q, want %q", got[0].Command, script)
	}

	runs = 0
	got, err = extensionList(config.Config{}, root)
	if err != nil {
		t.Fatal(err)
	}
	if runs != 0 {
		t.Fatalf("repaired again: %d uv runs", runs)
	}
	if got[0].Command != script {
		t.Errorf("Command = %q, want %q", got[0].Command, script)
	}
}

func TestExtensionListIgnoresConfigWithoutInstall(t *testing.T) {
	got, err := extensionList(config.Config{
		Extensions: map[string]config.Extension{"echo": {}},
	}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("extensions = %+v, want none", got)
	}
}

func TestExtensionListSkipsDisabled(t *testing.T) {
	root := t.TempDir()
	writeProject(t, root, "echo")
	off := false
	got, err := extensionList(config.Config{
		Extensions: map[string]config.Extension{
			"echo": {Enabled: &off},
		},
	}, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("extensions = %+v, want none", got)
	}
}

func TestExtensionListNameMismatch(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "echo")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(projectTOML("telegram")), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := extensionList(config.Config{}, root)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestServeStartsVenvExtension(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("OPENROUTER_API_KEY", "test")

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	if _, _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	extDir, err := config.ExtensionsDir()
	if err != nil {
		t.Fatal(err)
	}
	dir := writeProject(t, extDir, "echo")
	script := filepath.Join(dir, ".venv", "bin", "echo")
	if err := os.MkdirAll(filepath.Dir(script), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf ok > marker\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeConfig(t, config.Config{Listen: addr})

	app, err := loadApp()
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errc := make(chan error, 1)
	go func() { errc <- serve(ctx, app, addr) }()

	deadline := time.Now().Add(5 * time.Second)
	var got string
	for {
		data, err := os.ReadFile(filepath.Join(dir, "marker"))
		if err == nil && len(data) > 0 {
			got = strings.TrimSpace(string(data))
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("marker = %q (%v), want venv script launched", got, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if got != "ok" {
		t.Fatalf("marker = %q, want ok", got)
	}

	cancel()
	if err := <-errc; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func writeProject(t *testing.T, root, name string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(projectTOML(name)), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func writeVenvEcho(t *testing.T, dir string) string {
	t.Helper()
	script := filepath.Join(dir, ".venv", "bin", "echo")
	if err := os.MkdirAll(filepath.Dir(script), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return script
}

func projectTOML(name string) string {
	return "[project]\nname = \"" + name + "\"\nversion = \"0.1.0\"\n\n[project.scripts]\n" + name + " = \"" + name + ":main\"\n"
}

func writeConfig(t *testing.T, cfg config.Config) {
	t.Helper()
	dir, err := config.EtcDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}
