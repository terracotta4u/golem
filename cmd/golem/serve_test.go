package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/terracotta4u/golem/config"
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
	dir := filepath.Join(extDir, "echo")
	out := filepath.Join(t.TempDir(), "env")
	writeManifest(t, extDir, "echo", `{"name":"echo","version":"0.1.0","kind":"channel","command":"./run"}`)
	if err := os.WriteFile(filepath.Join(dir, "run"), []byte("#!/bin/sh\nprintf '%s %s' \"$GOLEM_URL\" \"$GOLEM_TOKEN\" > "+strconv.Quote(out)+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeConfig(t, config.Config{
		Listen:     addr,
		Extensions: map[string]config.Extension{"echo": {}},
	})

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

func TestExtensionListFillsFromManifest(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "echo", `{"name":"echo","version":"0.1.0","kind":"channel","command":"./run","args":["--poll"]}`)

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
	if ext.Name != "echo" || ext.Command != "./run" || ext.Dir != filepath.Join(root, "echo") {
		t.Errorf("extension = %+v", ext)
	}
	if len(ext.Args) != 1 || ext.Args[0] != "--poll" {
		t.Errorf("Args = %q", ext.Args)
	}
	if ext.Env["TOKEN"] != "x" {
		t.Errorf("Env = %v", ext.Env)
	}
}

func TestExtensionListResolvesVenvCommand(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "echo", `{"name":"echo","version":"0.1.0","kind":"channel","command":"echo","args":["--poll"]}`)
	dir := filepath.Join(root, "echo")
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]\nname = \"echo\"\nversion = \"0.1.0\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(dir, ".venv", "bin", "echo")
	if err := os.MkdirAll(filepath.Dir(script), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	got, err := extensionList(config.Config{
		Extensions: map[string]config.Extension{"echo": {}},
	}, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("extensions = %d, want 1", len(got))
	}
	if got[0].Command != script {
		t.Errorf("Command = %q, want %q", got[0].Command, script)
	}
	if len(got[0].Args) != 1 || got[0].Args[0] != "--poll" {
		t.Errorf("Args = %q, want [--poll]", got[0].Args)
	}
}

func TestExtensionListRequiresInstall(t *testing.T) {
	_, err := extensionList(config.Config{
		Extensions: map[string]config.Extension{"echo": {}},
	}, t.TempDir())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not installed") {
		t.Errorf("error = %v, want not installed", err)
	}
}

func TestExtensionListSkipsDisabled(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "echo", `{"name":"echo","version":"0.1.0","kind":"channel","command":"./run"}`)
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
	writeManifest(t, root, "echo", `{"name":"telegram","version":"0.1.0","kind":"channel","command":"./run"}`)

	_, err := extensionList(config.Config{
		Extensions: map[string]config.Extension{"echo": {}},
	}, root)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestServeStartsExtensionFromManifest(t *testing.T) {
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
	dir := filepath.Join(extDir, "echo")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "golem.json"), []byte(`{"name":"echo","version":"0.1.0","kind":"channel","command":"./run"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "run"), []byte("#!/bin/sh\nprintf ok > marker\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeConfig(t, config.Config{
		Listen:     addr,
		Extensions: map[string]config.Extension{"echo": {}},
	})

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
			t.Fatalf("marker = %q (%v), want extension launched from manifest", got, err)
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
	dir := filepath.Join(extDir, "echo")
	writeManifest(t, extDir, "echo", `{"name":"echo","version":"0.1.0","kind":"channel","command":"echo"}`)
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]\nname = \"echo\"\nversion = \"0.1.0\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(dir, ".venv", "bin", "echo")
	if err := os.MkdirAll(filepath.Dir(script), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf ok > marker\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeConfig(t, config.Config{
		Listen:     addr,
		Extensions: map[string]config.Extension{"echo": {}},
	})

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

func writeManifest(t *testing.T, root, name, contents string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "golem.json"), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
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
