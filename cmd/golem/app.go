package main

import (
	"fmt"
	"os"

	"github.com/terracotta4u/golem/agent"
	"github.com/terracotta4u/golem/config"
	"github.com/terracotta4u/golem/conversation"
	"github.com/terracotta4u/golem/memory"
	"github.com/terracotta4u/golem/registry"
	"github.com/terracotta4u/golem/tool"
)

type app struct {
	cfg           config.Config
	conversations *conversation.DB
	agent         *agent.Agent
	reg           *registry.Registry
}

// setup loads ~/.golem, opens the conversation database, and reports first-run creation.
func setup() (config.Config, *conversation.DB, error) {
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
	conversations, err := conversation.Open(dir)
	if err != nil {
		return config.Config{}, nil, err
	}
	return cfg, conversations, nil
}

func loadApp() (*app, error) {
	cfg, conversations, err := setup()
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			conversations.Close()
		}
	}()

	tools := tool.Builtins()

	reg := registry.New("")

	a := agent.New(registry.BindChat(reg, configModel("default")), tools...)
	a.Fast = registry.BindChat(reg, configModel("fast"))
	a.MaxToolRounds = cfg.MaxToolRounds
	attachMemory(a, reg)
	ok = true
	return &app{cfg: cfg, conversations: conversations, agent: a, reg: reg}, nil
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

func attachMemory(a *agent.Agent, reg *registry.Registry) {
	cfg, _, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "memory: %v\n", err)
		return
	}
	mem := cfg.Memory
	if mem == nil {
		return
	}

	path, err := config.MemoriesDB()
	if err != nil {
		fmt.Fprintf(os.Stderr, "memory: %v\n", err)
		return
	}
	st, err := memory.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "memory: open: %v\n", err)
		return
	}
	a.Memories.Store = st
	a.Memories.MinSimilarity = mem.MinSimilarity
	a.Memories.BudgetTokens = mem.BudgetTokens

	emb := registry.BindEmbed(reg, configModel("embed"))
	idx, err := memory.NewIndex(st, emb, mem.Embedding.Provider, mem.Embedding.Model)
	if err != nil {
		fmt.Fprintf(os.Stderr, "memory: index: %v\n", err)
		return
	}
	a.Memories.Search = idx
	a.Memories.Index = idx
}
