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

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		apiKey = cfg.APIKey
	}
	if apiKey == "" {
		return nil, fmt.Errorf("set api_key in ~/.golem/etc/conf.json or OPENROUTER_API_KEY")
	}

	model := os.Getenv("OPENROUTER_MODEL")
	if model == "" {
		model = cfg.Model
	}
	if model == "" {
		model = "openai/gpt-4o-mini"
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
	a := agent.New(openrouter.New(apiKey, model), dir, tools...)
	a.MaxToolRounds = cfg.MaxToolRounds
	attachMemory(a, cfg, apiKey)
	return &app{cfg: cfg, store: st, agent: a}, nil
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
