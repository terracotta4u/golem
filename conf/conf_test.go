package conf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCreatesConf(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg, created, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("expected first load to create conf")
	}
	if cfg.DefaultModel.Provider != "openrouter" || cfg.DefaultModel.Model != "openai/gpt-4o-mini" {
		t.Errorf("default = %+v", cfg.DefaultModel)
	}
	if cfg.FastModel.Provider != "openrouter" || cfg.FastModel.Model != "openai/gpt-4o-mini" {
		t.Errorf("fast = %+v", cfg.FastModel)
	}
	if cfg.Memory.Embedding.Provider != "openrouter" || cfg.Memory.Embedding.Model != "openai/text-embedding-3-small" {
		t.Errorf("embedding = %+v", cfg.Memory.Embedding)
	}
	if cfg.Memory.BudgetTokens != 800 || cfg.Memory.MinSimilarity != 0.5 {
		t.Errorf("budget/min = %d/%v", cfg.Memory.BudgetTokens, cfg.Memory.MinSimilarity)
	}

	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "conf.json")); err != nil {
		t.Fatal(err)
	}

	cfg2, created2, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if created2 {
		t.Fatal("second load should not create conf")
	}
	if cfg2.DefaultModel != cfg.DefaultModel || cfg2.FastModel != cfg.FastModel {
		t.Errorf("cfg2 = %+v", cfg2)
	}

	data, err := os.ReadFile(filepath.Join(dir, fileName))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxToolRounds != 0 {
		t.Errorf("MaxToolRounds = %d, want 0 (no cap)", cfg.MaxToolRounds)
	}
	if strings.Contains(string(data), "max_tool_rounds") {
		t.Errorf("default conf should omit max_tool_rounds: %s", data)
	}
	if strings.Contains(string(data), "listen") {
		t.Errorf("default conf should omit listen: %s", data)
	}
	if !strings.Contains(string(data), `"default_model"`) ||
		!strings.Contains(string(data), `"fast_model"`) ||
		!strings.Contains(string(data), `"memory"`) ||
		!strings.Contains(string(data), `"openai/text-embedding-3-small"`) ||
		!strings.Contains(string(data), `"budget_tokens"`) ||
		!strings.Contains(string(data), `"min_similarity"`) {
		t.Errorf("default conf should include models and memory knobs: %s", data)
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

func TestRuntimeDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := RuntimeDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "runtime") {
		t.Errorf("RuntimeDir = %q, want %s/runtime", got, dir)
	}
}

func TestUVDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir, err := RuntimeDir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := UVDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "uv") {
		t.Errorf("UVDir = %q, want %s/uv", got, dir)
	}
}

func TestUVCacheDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir, err := RuntimeDir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := UVCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "cache") {
		t.Errorf("UVCacheDir = %q, want %s/cache", got, dir)
	}
}

func TestUVPythonDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir, err := RuntimeDir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := UVPythonDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "python") {
		t.Errorf("UVPythonDir = %q, want %s/python", got, dir)
	}
}

func TestSaveRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := Load(); err != nil {
		t.Fatal(err)
	}
	cfg := Conf{Extensions: map[string]Extension{
		"echo": {Source: "/tmp/echo", Ref: "HEAD", Revision: "abc123"},
	}}
	cfg.DefaultModel.Model = "test-model"
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	got, created, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("Save should not look like first-run create")
	}
	if got.DefaultModel.Model != "test-model" {
		t.Errorf("got = %+v", got)
	}
	e := got.Extensions["echo"]
	if e.Source != "/tmp/echo" || e.Ref != "HEAD" || e.Revision != "abc123" {
		t.Errorf("origin = %+v", e)
	}
}

func TestLoadFillsEmbeddingWhenOmitted(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := Load(); err != nil {
		t.Fatal(err)
	}
	partial := Conf{DefaultModel: ModelConfig{Model: "test-model"}}
	if err := Save(partial); err != nil {
		t.Fatal(err)
	}

	got, created, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("conf already existed")
	}
	if got.DefaultModel.Provider != "openrouter" {
		t.Errorf("default provider = %q, want openrouter", got.DefaultModel.Provider)
	}
	if got.Memory.Embedding.Provider != "openrouter" || got.Memory.Embedding.Model != "openai/text-embedding-3-small" {
		t.Errorf("embedding = %+v", got.Memory.Embedding)
	}
}

func TestLoadWritesMigratedConfToDisk(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := Load(); err != nil {
		t.Fatal(err)
	}
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, fileName)
	old := `{
  "extensions": {
    "golem-telegram": {
      "source": "https://github.com/terracotta4u/golem-telegram",
      "ref": "HEAD"
    }
  }
}
`
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, _, err := Load(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, `"default_model"`) || !strings.Contains(s, `"fast_model"`) {
		t.Errorf("file missing migrated models: %s", s)
	}
	if !strings.Contains(s, `"memory"`) ||
		!strings.Contains(s, `"openai/text-embedding-3-small"`) ||
		!strings.Contains(s, `"budget_tokens"`) {
		t.Errorf("file missing migrated memory: %s", s)
	}
	if !strings.Contains(s, `"golem-telegram"`) {
		t.Errorf("file dropped extensions: %s", s)
	}
}

func TestSaveMemoryEmbeddingRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := Load(); err != nil {
		t.Fatal(err)
	}
	cfg := Conf{
		DefaultModel: ModelConfig{Model: "test-model"},
		Memory: &MemoryConfig{
			Embedding: ModelConfig{Provider: "ollama", Model: "nomic-embed-text"},
		},
	}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	got, created, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("Save should not look like first-run create")
	}
	if got.Memory.Embedding.Provider != "ollama" || got.Memory.Embedding.Model != "nomic-embed-text" {
		t.Errorf("embedding = %+v", got.Memory.Embedding)
	}
}

func TestLoadMaxToolRounds(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := Load(); err != nil {
		t.Fatal(err)
	}
	rounds := Conf{DefaultModel: ModelConfig{Model: "test-model"}, MaxToolRounds: 20}
	if err := Save(rounds); err != nil {
		t.Fatal(err)
	}
	got, created, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("Save should not look like first-run create")
	}
	if got.MaxToolRounds != 20 {
		t.Errorf("MaxToolRounds = %d, want 20", got.MaxToolRounds)
	}
}

func TestRemoveExtensionDeletesEntry(t *testing.T) {
	cfg := Conf{
		Extensions: map[string]Extension{
			"echo":     {Env: map[string]string{"ECHO_TOKEN": "x"}},
			"telegram": {},
		},
	}
	RemoveExtension(&cfg, "echo")
	if _, ok := cfg.Extensions["echo"]; ok {
		t.Fatal("echo still in conf")
	}
	if _, ok := cfg.Extensions["telegram"]; !ok {
		t.Fatal("removed telegram")
	}
}

func TestSetExtensionOriginWritesFields(t *testing.T) {
	var cfg Conf
	SetExtensionOrigin(&cfg, "echo", "/tmp/echo", "HEAD", "abc123")
	got := cfg.Extensions["echo"]
	if got.Source != "/tmp/echo" || got.Ref != "HEAD" || got.Revision != "abc123" {
		t.Errorf("got = %+v", got)
	}
}

func TestSetExtensionOriginKeepsEnv(t *testing.T) {
	cfg := Conf{
		Extensions: map[string]Extension{
			"echo": {Env: map[string]string{"ECHO_TOKEN": "secret"}},
		},
	}
	SetExtensionOrigin(&cfg, "echo", "/tmp/echo", "", "")
	got := cfg.Extensions["echo"]
	if got.Source != "/tmp/echo" {
		t.Errorf("source = %q", got.Source)
	}
	if got.Env["ECHO_TOKEN"] != "secret" {
		t.Errorf("wiped env: %+v", got)
	}
}
