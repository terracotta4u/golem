package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCreatesConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg, created, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("expected first load to create config")
	}
	if cfg.Provider != "openrouter" || cfg.Model != "openai/gpt-4o-mini" || cfg.Listen != DefaultListen {
		t.Errorf("cfg = %+v", cfg)
	}

	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "config.json")); err != nil {
		t.Fatal(err)
	}

	cfg2, created2, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if created2 {
		t.Fatal("second load should not create config")
	}
	if cfg2.Provider != cfg.Provider || cfg2.Model != cfg.Model || cfg2.Listen != cfg.Listen {
		t.Errorf("cfg2 = %+v", cfg2)
	}
}

func TestSkillsDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := SkillsDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "skills") {
		t.Errorf("SkillsDir = %q, want %s/skills", got, dir)
	}
}
