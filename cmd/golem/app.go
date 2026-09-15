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
	hub   *provider.Hub
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

	hub := provider.NewHub(0)
	key := os.Getenv("OPENROUTER_API_KEY")
	if key == "" && usesOpenRouter(cfg) {
		return nil, fmt.Errorf("set OPENROUTER_API_KEY")
	}
	if key != "" {
		if err := seedOpenRouter(hub, key); err != nil {
			return nil, err
		}
	}

	a := agent.New(provider.NewLazyChat(hub, confModel("default")), dir, tools...)
	a.Fast = provider.NewLazyChat(hub, confModel("fast"))
	a.MaxToolRounds = cfg.MaxToolRounds
	attachMemory(a, hub)
	return &app{cfg: cfg, store: st, agent: a, hub: hub}, nil
}

func usesOpenRouter(cfg conf.Conf) bool {
	if cfg.DefaultModel.Provider == "openrouter" || cfg.FastModel.Provider == "openrouter" {
		return true
	}
	return cfg.Memory != nil && cfg.Memory.Embedding.Provider == "openrouter"
}

func seedOpenRouter(hub *provider.Hub, key string) error {
	return hub.Register("openrouter", provider.Backend{
		Chat: provider.NewModelChat(func(model string) provider.Provider {
			return openrouter.New(key, model)
		}),
		Embedder: provider.NewModelEmbedder(func(model string) provider.Embedder {
			return openrouter.NewEmbedder(key, model)
		}),
	})
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

func attachMemory(a *agent.Agent, hub *provider.Hub) {
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

	emb := provider.NewLazyEmbedder(hub, confModel("embed"))
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
