package main

import (
	"fmt"
	"os"

	"github.com/terracotta4u/golem/agent"
	"github.com/terracotta4u/golem/conf"
	"github.com/terracotta4u/golem/memory"
	"github.com/terracotta4u/golem/provider"
	"github.com/terracotta4u/golem/provider/openrouter"
	"github.com/terracotta4u/golem/skill"
	"github.com/terracotta4u/golem/store"
	"github.com/terracotta4u/golem/tool"
)

type app struct {
	cfg   conf.Conf
	store store.Store
	agent *agent.Agent
}

// setup loads ~/.golem, opens the file store, and reports first-run creation.
func setup() (conf.Conf, store.Store, error) {
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
	st, err := store.NewFileStore(dir)
	if err != nil {
		return conf.Conf{}, nil, err
	}
	return cfg, st, nil
}

func loadApp() (*app, error) {
	cfg, st, err := setup()
	if err != nil {
		return nil, err
	}

	skills, err := loadSkills()
	if err != nil {
		return nil, err
	}

	tools := []tool.Tool{
		tool.NewRead(),
		tool.NewWrite(),
		tool.NewEdit(),
		tool.NewShell(),
	}
	if len(skills) > 0 {
		tools = append(tools, tool.NewSkill(skills))
	}

	dir, err := conf.EtcDir()
	if err != nil {
		return nil, err
	}

	envKey := os.Getenv("OPENROUTER_API_KEY")
	reg := provider.NewRegistry()
	reg.RegisterChat("openrouter", func(model, apiKey string) (provider.Provider, error) {
		if apiKey == "" {
			return nil, fmt.Errorf("set OPENROUTER_API_KEY")
		}
		return openrouter.New(apiKey, model), nil
	})

	defaultModel := os.Getenv("OPENROUTER_MODEL")
	if defaultModel == "" {
		defaultModel = cfg.DefaultModel.Model
	}
	if defaultModel == "" {
		defaultModel = "openai/gpt-4o-mini"
	}

	defaultP, err := reg.Chat(cfg.DefaultModel.Provider, defaultModel, chatAPIKey(cfg.DefaultModel.Provider, envKey))
	if err != nil {
		return nil, err
	}
	fastP, err := reg.Chat(cfg.FastModel.Provider, cfg.FastModel.Model, chatAPIKey(cfg.FastModel.Provider, envKey))
	if err != nil {
		return nil, err
	}

	a := agent.New(defaultP, dir, tools...)
	a.Fast = fastP
	a.MaxToolRounds = cfg.MaxToolRounds
	attachMemory(a, cfg, envKey)
	return &app{cfg: cfg, store: st, agent: a}, nil
}

func chatAPIKey(name, envKey string) string {
	if name == "openrouter" {
		return envKey
	}
	return ""
}

func attachMemory(a *agent.Agent, cfg conf.Conf, apiKey string) {
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

	mem := cfg.Memory
	if mem == nil {
		return
	}
	a.MinSimilarity = mem.MinSimilarity
	a.BudgetTokens = mem.BudgetTokens

	reg := provider.NewRegistry()
	reg.RegisterEmbedder("openrouter", func(model string) (provider.Embedder, error) {
		return openrouter.NewEmbedder(apiKey, model), nil
	})

	name, model := mem.Embedding.Provider, mem.Embedding.Model
	emb, err := reg.Embedder(name, model)
	if err != nil {
		fmt.Fprintf(os.Stderr, "memory: embedder: %v\n", err)
		return
	}
	idx, err := memory.NewIndex(st, emb, name, model)
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
