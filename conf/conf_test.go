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
	if cfg.Provider != "openrouter" || cfg.Model != "openai/gpt-4o-mini" {
		t.Errorf("cfg = %+v", cfg)
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
	if _, err := os.Stat(filepath.Join(dir, "etc", "conf.json")); err != nil {
		t.Fatal(err)
	}

	cfg2, created2, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if created2 {
		t.Fatal("second load should not create conf")
	}
	if cfg2.Provider != cfg.Provider || cfg2.Model != cfg.Model {
		t.Errorf("cfg2 = %+v", cfg2)
	}

	etc, err := EtcDir()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(etc, fileName))
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
	if !strings.Contains(string(data), `"memory"`) ||
		!strings.Contains(string(data), `"openai/text-embedding-3-small"`) ||
		!strings.Contains(string(data), `"budget_tokens"`) ||
		!strings.Contains(string(data), `"min_similarity"`) {
		t.Errorf("default conf should include memory embedding and retrieval knobs: %s", data)
	}
}

func TestLoadCreatesIdentityFiles(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if _, _, err := Load(); err != nil {
		t.Fatal(err)
	}
	etc, err := EtcDir()
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"SOUL.md", "USER.md"} {
		data, err := os.ReadFile(filepath.Join(etc, name))
		if err != nil {
			t.Fatal(err)
		}
		if len(data) != 0 {
			t.Errorf("%s = %q, want empty", name, data)
		}
	}

	if err := os.WriteFile(filepath.Join(etc, "SOUL.md"), []byte("custom soul\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(etc, "SOUL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "custom soul\n" {
		t.Errorf("SOUL.md overwritten: %q", got)
	}
}

func TestLoadCreatesMissingIdentityFilesWhenConfExists(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if _, _, err := Load(); err != nil {
		t.Fatal(err)
	}
	etc, err := EtcDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(etc, "USER.md")); err != nil {
		t.Fatal(err)
	}

	if _, created, err := Load(); err != nil {
		t.Fatal(err)
	} else if created {
		t.Fatal("conf already existed")
	}
	if _, err := os.Stat(filepath.Join(etc, "USER.md")); err != nil {
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

func TestEtcDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := EtcDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "etc") {
		t.Errorf("EtcDir = %q, want %s/etc", got, dir)
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
	cfg := Conf{Model: "test-model", Extensions: map[string]Extension{
		"echo": {Source: "/tmp/echo", Ref: "HEAD", Revision: "abc123"},
	}}
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
	if got.Model != "test-model" {
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
	if err := Save(Conf{Model: "test-model"}); err != nil {
		t.Fatal(err)
	}

	got, created, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("conf already existed")
	}
	if got.Provider != "openrouter" {
		t.Errorf("provider = %q, want openrouter", got.Provider)
	}
	if got.Memory.Embedding.Provider != "openrouter" || got.Memory.Embedding.Model != "openai/text-embedding-3-small" {
		t.Errorf("embedding = %+v", got.Memory.Embedding)
	}
}

func TestSaveMemoryEmbeddingRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := Load(); err != nil {
		t.Fatal(err)
	}
	cfg := Conf{
		Model: "test-model",
		Memory: &MemoryConfig{
			Embedding: EmbeddingConfig{Provider: "ollama", Model: "nomic-embed-text"},
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
	if err := Save(Conf{Model: "test-model", MaxToolRounds: 20}); err != nil {
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
