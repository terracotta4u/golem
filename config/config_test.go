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

func TestLoadCreatesIdentityFiles(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if _, _, err := Load(); err != nil {
		t.Fatal(err)
	}
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"SOUL.md", "USER.md"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if len(data) != 0 {
			t.Errorf("%s = %q, want empty", name, data)
		}
	}

	if err := os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte("custom soul\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "SOUL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "custom soul\n" {
		t.Errorf("SOUL.md overwritten: %q", got)
	}
}

func TestLoadCreatesMissingIdentityFilesWhenConfigExists(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if _, _, err := Load(); err != nil {
		t.Fatal(err)
	}
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "USER.md")); err != nil {
		t.Fatal(err)
	}

	if _, created, err := Load(); err != nil {
		t.Fatal(err)
	} else if created {
		t.Fatal("config already existed")
	}
	if _, err := os.Stat(filepath.Join(dir, "USER.md")); err != nil {
		t.Fatal(err)
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

func TestExtensionsDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := ExtensionsDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "extensions") {
		t.Errorf("ExtensionsDir = %q, want %s/extensions", got, dir)
	}
}
