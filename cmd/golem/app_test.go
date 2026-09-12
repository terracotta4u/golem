package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/terracotta4u/golem/conf"
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
	t.Setenv("OPENROUTER_API_KEY", "test")
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
	if app.agent.MemoryStore == nil {
		t.Fatal("MemoryStore is nil")
	}
	if app.agent.Memory == nil {
		t.Fatal("Memory is nil")
	}
	if app.agent.Indexer == nil {
		t.Fatal("Indexer is nil")
	}
	if app.agent.MinSimilarity != 0.5 {
		t.Errorf("MinSimilarity = %v, want 0.5", app.agent.MinSimilarity)
	}
	if app.agent.BudgetTokens != 800 {
		t.Errorf("BudgetTokens = %d, want 800", app.agent.BudgetTokens)
	}
}

func TestLoadAppUnknownEmbedderIsStoreOnly(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("OPENROUTER_API_KEY", "test")
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
	if app.agent.MemoryStore == nil {
		t.Fatal("MemoryStore is nil, want store-only")
	}
	if app.agent.Memory != nil {
		t.Error("Memory set, want nil when embedder resolve fails")
	}
	if app.agent.Indexer != nil {
		t.Error("Indexer set, want nil when embedder resolve fails")
	}
}
