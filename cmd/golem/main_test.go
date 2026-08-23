package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/terracotta4u/golem/config"
	"github.com/terracotta4u/golem/daemon"
)

func TestChatStartsWhenNotRunning(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	writeListen(t, "127.0.0.1:1")

	started := false
	origStart := daemon.StartFn
	daemon.StartFn = func() error {
		started = true
		return fmt.Errorf("refused")
	}
	t.Cleanup(func() { daemon.StartFn = origStart })

	err := run([]string{})
	if !started {
		t.Fatal("expected chat to start serve when not running")
	}
	if err == nil {
		t.Fatal("expected start error")
	}
	if !strings.Contains(err.Error(), "start golem") {
		t.Fatalf("error = %q, want it to mention start golem", err)
	}
}

func writeListen(t *testing.T, listen string) {
	t.Helper()
	dir, err := config.Dir()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg config.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	cfg.Listen = listen
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(out, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestRunServeNeedsAPIKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("OPENROUTER_API_KEY", "")

	err := run([]string{"serve"})
	if err == nil {
		t.Fatal("expected error when serve has no API key")
	}
	if !strings.Contains(err.Error(), "api_key") && !strings.Contains(err.Error(), "OPENROUTER_API_KEY") {
		t.Fatalf("error = %q, want it to mention the API key", err)
	}
}
