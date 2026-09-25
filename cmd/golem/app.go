package main

import (
	"fmt"
	"os"

	"github.com/terracotta4u/golem/agent"
	"github.com/terracotta4u/golem/conf"
	"github.com/terracotta4u/golem/conversation"
	"github.com/terracotta4u/golem/memory"
	"github.com/terracotta4u/golem/provider"
	"github.com/terracotta4u/golem/registry"
	"github.com/terracotta4u/golem/skill"
	"github.com/terracotta4u/golem/tool"
)

type app struct {
	cfg           conf.Conf
	conversations *conversation.DB
	agent         *agent.Agent
	hub           *provider.Hub
	reg           *registry.Registry
}

// setup loads ~/.golem, opens the conversation database, and reports first-run creation.
func setup() (conf.Conf, *conversation.DB, error) {
	cfg, created, err := conf.Load()
	if err != nil {
		return conf.Conf{}, nil, err
	}
	dir, err := conf.Dir()
	if err != nil {
		return conf.Conf{}, nil, err
	}
	if created {
		fmt.Fprintf(os.Stderr, "created %s\n", dir)
	}
	conversations, err := conversation.Open(dir)
	if err != nil {
		return conf.Conf{}, nil, err
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

	skills, err := loadSkills()
	if err != nil {
		return nil, err
	}

	tools := tool.Builtins(skills)

	dir, err := conf.Dir()
	if err != nil {
		return nil, err
	}

	hub := provider.NewHub(0)
	reg := registry.New(hub, "")

	a := agent.New(registry.BindChat(reg, confModel("default")), dir, tools...)
	a.Fast = registry.BindChat(reg, confModel("fast"))
	a.MaxToolRounds = cfg.MaxToolRounds
	attachMemory(a, reg)
	ok = true
	return &app{cfg: cfg, conversations: conversations, agent: a, hub: hub, reg: reg}, nil
}

func confModel(which string) func() (string, string, error) {
	return func() (string, string, error) {
		cfg, _, err := conf.Load()
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
	path, err := conf.MemoriesDB()
	if err != nil {
		fmt.Fprintf(os.Stderr, "memory: %v\n", err)
		return
	}
	st, err := memory.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "memory: open: %v\n", err)
		return
	}
	a.MemoryStore = st

	cfg, _, err := conf.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "memory: %v\n", err)
		return
	}
	mem := cfg.Memory
	if mem == nil {
		return
	}
	a.MinSimilarity = mem.MinSimilarity
	a.BudgetTokens = mem.BudgetTokens

	emb := registry.BindEmbed(reg, confModel("embed"))
	idx, err := memory.NewIndex(st, emb, mem.Embedding.Provider, mem.Embedding.Model)
	if err != nil {
		fmt.Fprintf(os.Stderr, "memory: index: %v\n", err)
		return
	}
	a.Memory = idx
	a.Indexer = idx
}

func loadSkills() ([]skill.Skill, error) {
	dir, err := conf.SkillsDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create %s: %w", dir, err)
	}
	return skill.LoadDir(dir)
}
