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

func TestServeStartsConfiguredChannel(t *testing.T) {
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
	out := filepath.Join(t.TempDir(), "env")
	writeConfig(t, config.Config{
		Listen: addr,
		Channels: map[string]config.Channel{
			"echo": {
				Command: "sh",
				Args:    []string{"-c", "printf '%s %s' \"$GOLEM_URL\" \"$GOLEM_TOKEN\" > " + strconv.Quote(out)},
			},
		},
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
			t.Fatalf("channel env = %q (%v), want GOLEM_URL=http://%s and a token", got, err, addr)
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
		Channels: map[string]config.Channel{
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

func TestExtensionListConfigOverridesManifest(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "echo", `{"name":"echo","version":"0.1.0","kind":"channel","command":"./run","args":["--poll"]}`)

	got, err := extensionList(config.Config{
		Channels: map[string]config.Channel{
			"echo": {Command: "sh", Args: []string{"-c", "true"}},
		},
	}, root)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Command != "sh" || len(got[0].Args) != 2 || got[0].Dir != filepath.Join(root, "echo") {
		t.Errorf("extension = %+v", got[0])
	}
}

func TestExtensionListKeepsPATHChannel(t *testing.T) {
	got, err := extensionList(config.Config{
		Channels: map[string]config.Channel{
			"echo": {Command: "sh", Args: []string{"-c", "true"}},
		},
	}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Command != "sh" || got[0].Dir != "" {
		t.Errorf("extension = %+v, want PATH channel with empty Dir", got[0])
	}
}

func TestExtensionListNameMismatch(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "echo", `{"name":"telegram","version":"0.1.0","kind":"channel","command":"./run"}`)

	_, err := extensionList(config.Config{
		Channels: map[string]config.Channel{"echo": {}},
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
		Listen:   addr,
		Channels: map[string]config.Channel{"echo": {}},
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
	dir, err := config.Dir()
	if err != nil {
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
