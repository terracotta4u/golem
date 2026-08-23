package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	dirName       = ".golem"
	fileName      = "config.json"
	DefaultListen = "127.0.0.1:8743"
)

type Config struct {
	Provider string             `json:"provider,omitempty"`
	Model    string             `json:"model"`
	APIKey   string             `json:"api_key,omitempty"`
	Listen   string             `json:"listen,omitempty"`
	Channels map[string]Channel `json:"channels,omitempty"`
}

type Channel struct {
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

func defaults() Config {
	return Config{
		Provider: "openrouter",
		Model:    "openai/gpt-4o-mini",
		Listen:   DefaultListen,
	}
}

// Dir is ~/.golem.
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home directory: %w", err)
	}
	return filepath.Join(home, dirName), nil
}

// ChannelsDir is ~/.golem/channels.
func ChannelsDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "channels"), nil
}

func DaemonPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "daemon.json"), nil
}

func LogPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "golem.log"), nil
}

// Load creates ~/.golem and a default config on first run, then reads the config.
// created is true when the config file did not already exist.
func Load() (cfg Config, created bool, err error) {
	dir, err := Dir()
	if err != nil {
		return Config{}, false, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Config{}, false, fmt.Errorf("create %s: %w", dir, err)
	}

	path := filepath.Join(dir, fileName)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		cfg := defaults()
		if err := write(path, cfg); err != nil {
			return Config{}, false, err
		}
		return cfg, true, nil
	}
	if err != nil {
		return Config{}, false, fmt.Errorf("read %s: %w", path, err)
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, false, fmt.Errorf("parse %s: %w", path, err)
	}
	if cfg.Provider == "" {
		cfg.Provider = defaults().Provider
	}
	if cfg.Listen == "" {
		cfg.Listen = DefaultListen
	}
	return cfg, false, nil
}

func write(path string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
