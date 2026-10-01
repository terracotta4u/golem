package conf

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

const (
	dirName  = ".golem"
	fileName = "conf.json"
)

type Conf struct {
	DefaultModel  ModelConfig          `json:"default_model"`
	FastModel     ModelConfig          `json:"fast_model"`
	MaxToolRounds int                  `json:"max_tool_rounds,omitempty"`
	Memory        *MemoryConfig        `json:"memory,omitempty"`
	Extensions    map[string]Extension `json:"extensions,omitempty"`
}

type ModelConfig struct {
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
}

type MemoryConfig struct {
	Embedding     ModelConfig `json:"embedding,omitempty"`
	BudgetTokens  int         `json:"budget_tokens,omitempty"`
	MinSimilarity float32     `json:"min_similarity,omitempty"`
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

// MemoryDir is ~/.golem/memory.
func MemoryDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "memory"), nil
}

// MemoriesDB is ~/.golem/memory/memories.db.
func MemoriesDB() (string, error) {
	dir, err := MemoryDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "memories.db"), nil
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

// Load creates ~/.golem and a default conf on first run, then reads the conf.
// created is true when the conf file did not already exist.
func Load() (cfg Conf, created bool, err error) {
	err = withLock(func() error {
		var loadErr error
		cfg, created, loadErr = loadUnlocked()
		return loadErr
	})
	return cfg, created, err
}

func loadUnlocked() (cfg Conf, created bool, err error) {
	path, err := filePath()
	if err != nil {
		return Conf{}, false, err
	}
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
	if migrate(&cfg) {
		if err := write(path, cfg); err != nil {
			return Conf{}, false, err
		}
	}
	return cfg, false, nil
}

func write(path string, cfg Conf) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode conf: %w", err)
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".conf.json.*")
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func Save(cfg Conf) error {
	return withLock(func() error {
		path, err := filePath()
		if err != nil {
			return err
		}
		return write(path, cfg)
	})
}

// Update reloads conf, applies fn, and writes it back while holding the
// conf lock, so a concurrent Save or Update cannot drop this edit.
func Update(fn func(*Conf) error) error {
	return withLock(func() error {
		cfg, _, err := loadUnlocked()
		if err != nil {
			return err
		}
		if err := fn(&cfg); err != nil {
			return err
		}
		path, err := filePath()
		if err != nil {
			return err
		}
		return write(path, cfg)
	})
}

func withLock(fn func() error) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "conf.json.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("lock conf: %w", err)
	}
	defer f.Close()
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		return fmt.Errorf("lock conf: %w", err)
	}
	defer unix.Flock(int(f.Fd()), unix.LOCK_UN)
	return fn()
}

func filePath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create %s: %w", dir, err)
	}
	return filepath.Join(dir, fileName), nil
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
