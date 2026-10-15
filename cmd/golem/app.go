package main

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/terracotta4u/golem/agent"
	"github.com/terracotta4u/golem/config"
	"github.com/terracotta4u/golem/conversation"
	"github.com/terracotta4u/golem/db"
	"github.com/terracotta4u/golem/memory"
	"github.com/terracotta4u/golem/registry"
	"github.com/terracotta4u/golem/tool"
)

type app struct {
	cfg           config.Config
	sqlDB         *sql.DB
	conversations *conversation.DB
	agent         *agent.Agent
	reg           *registry.Registry
}

func (a *app) Close() error {
	if a == nil || a.sqlDB == nil {
		return nil
	}
	return a.sqlDB.Close()
}

// setup loads ~/.golem, opens golem.db, and reports first-run creation.
func setup() (config.Config, *sql.DB, error) {
	cfg, created, err := config.Load()
	if err != nil {
		return config.Config{}, nil, err
	}
	dir, err := config.Dir()
	if err != nil {
		return config.Config{}, nil, err
	}
	if created {
		fmt.Fprintf(os.Stderr, "created %s\n", dir)
	}
	sqlDB, err := db.Open(dir)
	if err != nil {
		return config.Config{}, nil, err
	}
	return cfg, sqlDB, nil
}

func loadApp() (*app, error) {
	cfg, sqlDB, err := setup()
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			sqlDB.Close()
		}
	}()

	tools := tool.Builtins()

	reg := registry.New("")

	a := agent.New(registry.BindChat(reg, configModel("default")), tools...)
	a.Fast = registry.BindChat(reg, configModel("fast"))
	a.MaxToolRounds = cfg.MaxToolRounds
	attachMemory(a, reg, sqlDB)
	ok = true
	return &app{
		cfg:           cfg,
		sqlDB:         sqlDB,
		conversations: conversation.Share(sqlDB),
		agent:         a,
		reg:           reg,
	}, nil
}

func configModel(which string) func() (string, string, error) {
	return func() (string, string, error) {
		cfg, _, err := config.Load()
		if err != nil {
			return "", "", err
		}
		switch which {
		case "fast":
			return cfg.FastModel.Provider, cfg.FastModel.Model, nil
		case "embed":
			if cfg.Memory == nil {
				return "", "", fmt.Errorf("memory is not configured")
			}
			return cfg.Memory.Embedding.Provider, cfg.Memory.Embedding.Model, nil
		default:
			return cfg.DefaultModel.Provider, cfg.DefaultModel.Model, nil
		}
	}
}

func attachMemory(a *agent.Agent, reg *registry.Registry, sqlDB *sql.DB) {
	cfg, _, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "memory: %v\n", err)
		return
	}
	mem := cfg.Memory
	if mem == nil {
		return
	}

	st := memory.Share(sqlDB)
	a.Memories.Store = st
	a.Memories.MinSimilarity = mem.MinSimilarity
	a.Memories.BudgetTokens = mem.BudgetTokens

	emb := registry.BindEmbed(reg, configModel("embed"))
	idx := memory.NewIndex(st, emb, mem.Embedding.Provider, mem.Embedding.Model)
	a.Memories.Search = idx
	a.Memories.Index = idx
}
