package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/terracotta4u/golem/config"
	"github.com/terracotta4u/golem/conversation"
	"github.com/terracotta4u/golem/db"
	"github.com/terracotta4u/golem/memory"
	"github.com/terracotta4u/golem/provider"
)

func TestLoadAppWiresMemory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := config.Load(); err != nil {
		t.Fatal(err)
	}

	app, err := loadApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	dir, err := config.Dir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, db.FileName)); err != nil {
		t.Fatalf("golem.db: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "memory")); !os.IsNotExist(err) {
		t.Fatalf("memory dir: %v", err)
	}
	if app.agent.Memories.Store == nil {
		t.Fatal("Store is nil")
	}
	if app.agent.Memories.Search == nil {
		t.Fatal("Search is nil")
	}
	if app.agent.Memories.Index == nil {
		t.Fatal("Index is nil")
	}
	if app.agent.Memories.MinSimilarity != 0.5 {
		t.Errorf("MinSimilarity = %v, want 0.5", app.agent.Memories.MinSimilarity)
	}
	if app.agent.Memories.BudgetTokens != 800 {
		t.Errorf("BudgetTokens = %d, want 800", app.agent.Memories.BudgetTokens)
	}
}

func TestLoadAppSharesOneDatabase(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := config.Load(); err != nil {
		t.Fatal(err)
	}

	app, err := loadApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	conv := conversation.New("cli")
	conv.Title = "hello"
	conv.UpdatedAt = time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	conv.Messages = []provider.Message{{Role: "user", Content: "hi"}}
	if err := app.conversations.Save(conv); err != nil {
		t.Fatal(err)
	}
	if err := app.agent.Memories.Store.Save(memory.Memory{
		ID:             "mem-1",
		Content:        "User prefers tea.",
		ConversationID: conv.ID,
		TurnID:         "turn-1",
		CreatedAt:      conv.UpdatedAt,
	}); err != nil {
		t.Fatal(err)
	}

	var titles, contents int
	if err := app.sqlDB.QueryRow(`SELECT COUNT(*) FROM conversations WHERE id = ?`, conv.ID).Scan(&titles); err != nil {
		t.Fatal(err)
	}
	if err := app.sqlDB.QueryRow(`SELECT COUNT(*) FROM memories WHERE id = 'mem-1'`).Scan(&contents); err != nil {
		t.Fatal(err)
	}
	if titles != 1 || contents != 1 {
		t.Fatalf("conversations = %d, memories = %d, want 1 and 1", titles, contents)
	}
}

func TestLoadAppUnknownEmbedderWiresLazyIndex(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg, _, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Memory.Embedding.Provider = "ollama"
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	app, err := loadApp()
	if err != nil {
		t.Fatal(err)
	}
	if app.agent.Memories.Store == nil {
		t.Fatal("Store is nil")
	}
	if app.agent.Memories.Search == nil {
		t.Fatal("Search is nil")
	}
	if app.agent.Memories.Index == nil {
		t.Fatal("Index is nil")
	}
	_, err = app.agent.Memories.Search.Search(context.Background(), "hello", 1)
	if err == nil || !strings.Contains(err.Error(), "ollama") {
		t.Fatalf("search err = %v, want unknown ollama", err)
	}
}

func TestLoadAppSkipsDisabledMemory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg, _, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Memory = nil
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	app, err := loadApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	if app.agent.Memories.Store != nil || app.agent.Memories.Search != nil || app.agent.Memories.Index != nil {
		t.Fatal("memory was attached while disabled")
	}
	dir, err := config.Dir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, db.FileName)); err != nil {
		t.Fatalf("golem.db: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "memories.db")); !os.IsNotExist(err) {
		t.Fatalf("memories.db: %v", err)
	}
}

func TestLoadAppKeepsExplicitZeroMemory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg, _, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Memory.BudgetTokens = 0
	cfg.Memory.MinSimilarity = 0
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	app, err := loadApp()
	if err != nil {
		t.Fatal(err)
	}
	if app.agent.Memories.Store == nil {
		t.Fatal("Store is nil")
	}
	if app.agent.Memories.BudgetTokens != 0 || app.agent.Memories.MinSimilarity != 0 {
		t.Fatalf("budget/min = %d/%v, want 0/0", app.agent.Memories.BudgetTokens, app.agent.Memories.MinSimilarity)
	}
}

func TestLoadAppWiresFast(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := config.Load(); err != nil {
		t.Fatal(err)
	}

	app, err := loadApp()
	if err != nil {
		t.Fatal(err)
	}
	if app.agent.Fast == nil {
		t.Fatal("Fast is nil")
	}
}

func TestLoadAppUnknownChatProvider(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg, _, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.FastModel.Provider = "ollama"
	cfg.FastModel.Model = "llama3.2"
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	app, err := loadApp()
	if err != nil {
		t.Fatal(err)
	}
	_, err = app.agent.Fast.Chat(context.Background(), provider.ChatRequest{})
	if err == nil || !strings.Contains(err.Error(), "ollama") {
		t.Fatalf("fast err = %v, want unknown ollama", err)
	}
}
