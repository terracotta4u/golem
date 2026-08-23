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
