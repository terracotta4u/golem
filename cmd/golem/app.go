package main

import (
	"fmt"
	"os"

	"github.com/terracotta4u/golem/agent"
	"github.com/terracotta4u/golem/config"
	"github.com/terracotta4u/golem/provider/openrouter"
	"github.com/terracotta4u/golem/skill"
	"github.com/terracotta4u/golem/store"
	"github.com/terracotta4u/golem/tool"
)

type app struct {
	cfg   config.Config
	store store.Store
	agent *agent.Agent
}

// setup loads ~/.golem, opens the file store, and reports first-run creation.
func setup() (config.Config, store.Store, error) {
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
	st, err := store.NewFileStore(dir)
	if err != nil {
		return config.Config{}, nil, err
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
		return nil, fmt.Errorf("set api_key in ~/.golem/etc/config.json or OPENROUTER_API_KEY")
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

	dir, err := config.EtcDir()
	if err != nil {
		return nil, err
	}
	a := agent.New(openrouter.New(apiKey, model), dir, tools...)
	return &app{cfg: cfg, store: st, agent: a}, nil
}

func loadSkills() ([]skill.Skill, error) {
	dir, err := config.SkillsDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create %s: %w", dir, err)
	}
	return skill.LoadDir(dir)
}
