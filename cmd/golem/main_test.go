package main

import (
	"context"
	"strings"
	"testing"

	"github.com/terracotta4u/golem/conf"
	"github.com/terracotta4u/golem/provider"
)

func TestLoadAppWithoutAPIKey(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("OPENROUTER_API_KEY", "")
	if _, _, err := conf.Load(); err != nil {
		t.Fatal(err)
	}

	app, err := loadApp()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.hub.Get("openrouter"); err == nil {
		t.Fatal("hub should not seed openrouter")
	}
	_, err = app.agent.Fast.Chat(context.Background(), provider.ChatRequest{})
	if err == nil || !strings.Contains(err.Error(), "openrouter") {
		t.Fatalf("chat err = %v, want unknown openrouter", err)
	}
}
