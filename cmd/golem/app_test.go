package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/terracotta4u/golem/conf"
	"github.com/terracotta4u/golem/provider"
)

func TestLoadSkillsCreatesDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	skills, err := loadSkills()
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 0 {
		t.Errorf("skills = %+v, want empty", skills)
	}

	dir, err := conf.SkillsDir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatal(err)
	}
}

func TestLoadSkillsReadsSkillMD(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir, err := conf.SkillsDir()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "commit")
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte(`---
name: commit
description: Write commit messages.
---

Follow the commit format.
`), 0o600); err != nil {
		t.Fatal(err)
	}

	skills, err := loadSkills()
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 || skills[0].Name != "commit" {
		t.Errorf("skills = %+v, want [commit]", skills)
	}
}

func TestLoadAppWiresMemory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := conf.Load(); err != nil {
		t.Fatal(err)
	}

	app, err := loadApp()
	if err != nil {
		t.Fatal(err)
	}
	path, err := conf.MemoriesDB()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(filepath.Dir(path)) != "memory" || filepath.Base(path) != "memories.db" {
		t.Errorf("path = %q, want ~/.golem/memory/memories.db", path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("memories.db: %v", err)
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

func TestLoadAppUnknownEmbedderWiresLazyIndex(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg, _, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Memory.Embedding.Provider = "ollama"
	if err := conf.Save(cfg); err != nil {
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

func TestLoadAppWiresFast(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := conf.Load(); err != nil {
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
	cfg, _, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.FastModel.Provider = "ollama"
	cfg.FastModel.Model = "llama3.2"
	if err := conf.Save(cfg); err != nil {
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
