package conf

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	dirName  = ".golem"
	fileName = "conf.json"
	SoulFile = "SOUL.md"
	UserFile = "USER.md"
)

type Conf struct {
	Provider      string               `json:"provider,omitempty"`
	Model         string               `json:"model"`
	APIKey        string               `json:"api_key,omitempty"`
	MaxToolRounds int                  `json:"max_tool_rounds,omitempty"`
	Memory        *MemoryConfig        `json:"memory,omitempty"`
	Extensions    map[string]Extension `json:"extensions,omitempty"`
}

type MemoryConfig struct {
	Embedding EmbeddingConfig `json:"embedding,omitempty"`
}

type EmbeddingConfig struct {
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
}

type Extension struct {
	Source   string            `json:"source,omitempty"`
	Ref      string            `json:"ref,omitempty"`
	Revision string            `json:"revision,omitempty"`
	Env      map[string]string `json:"env,omitempty"`
}

// Dir is ~/.golem.
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home directory: %w", err)
	}
	return filepath.Join(home, dirName), nil
}

// EtcDir is ~/.golem/etc.
func EtcDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "etc"), nil
}

// ExtensionsDir is ~/.golem/extensions.
func ExtensionsDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "extensions"), nil
}

// SkillsDir is ~/.golem/skills.
func SkillsDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "skills"), nil
}

// RuntimeDir is ~/.golem/runtime.
func RuntimeDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "runtime"), nil
}

// UVDir is ~/.golem/runtime/uv.
func UVDir() (string, error) {
	dir, err := RuntimeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "uv"), nil
}

// UVCacheDir is ~/.golem/runtime/cache.
func UVCacheDir() (string, error) {
	dir, err := RuntimeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "cache"), nil
}

// UVPythonDir is ~/.golem/runtime/python.
func UVPythonDir() (string, error) {
	dir, err := RuntimeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "python"), nil
}

// Load creates ~/.golem/etc, SOUL.md, USER.md, and a default conf on first run,
// then reads the conf. created is true when the conf file did not already exist.
func Load() (cfg Conf, created bool, err error) {
	etc, err := EtcDir()
	if err != nil {
		return Conf{}, false, err
	}
	if err := os.MkdirAll(etc, 0o700); err != nil {
		return Conf{}, false, fmt.Errorf("create %s: %w", etc, err)
	}
	if err := ensureFile(filepath.Join(etc, SoulFile), ""); err != nil {
		return Conf{}, false, err
	}
	if err := ensureFile(filepath.Join(etc, UserFile), ""); err != nil {
		return Conf{}, false, err
	}

	path := filepath.Join(etc, fileName)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		cfg := defaults()
		if err := write(path, cfg); err != nil {
			return Conf{}, false, err
		}
		return cfg, true, nil
	}
	if err != nil {
		return Conf{}, false, fmt.Errorf("read %s: %w", path, err)
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return Conf{}, false, fmt.Errorf("parse %s: %w", path, err)
	}
	migrate(&cfg)
	return cfg, false, nil
}

func ensureFile(path, contents string) error {
	_, err := os.Stat(path)
	if err == nil {
		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func write(path string, cfg Conf) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode conf: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func Save(cfg Conf) error {
	etc, err := EtcDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(etc, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", etc, err)
	}
	return write(filepath.Join(etc, fileName), cfg)
}

func RemoveExtension(cfg *Conf, name string) {
	delete(cfg.Extensions, name)
}

func SetExtensionOrigin(cfg *Conf, name, source, ref, revision string) {
	if cfg.Extensions == nil {
		cfg.Extensions = make(map[string]Extension)
	}
	e := cfg.Extensions[name]
	e.Source = source
	e.Ref = ref
	e.Revision = revision
	cfg.Extensions[name] = e
}
