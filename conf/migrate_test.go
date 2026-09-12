package conf

import "testing"

func TestMigrateFillsMissingFields(t *testing.T) {
	cfg := Conf{Model: "custom-model"}
	migrate(&cfg)

	d := defaults()
	if cfg.Provider != d.Provider {
		t.Errorf("provider = %q, want %q", cfg.Provider, d.Provider)
	}
	if cfg.Model != "custom-model" {
		t.Errorf("model = %q, want custom-model", cfg.Model)
	}
	if cfg.Memory == nil {
		t.Fatal("memory is nil")
	}
	if cfg.Memory.Embedding.Provider != d.Memory.Embedding.Provider || cfg.Memory.Embedding.Model != d.Memory.Embedding.Model {
		t.Errorf("embedding = %+v", cfg.Memory.Embedding)
	}
}

func TestMigrateLeavesExplicitValues(t *testing.T) {
	cfg := Conf{
		Provider: "other",
		Model:    "custom-model",
		Memory: &MemoryConfig{
			Embedding: EmbeddingConfig{Provider: "ollama", Model: "nomic-embed-text"},
		},
	}
	migrate(&cfg)
	if cfg.Provider != "other" || cfg.Model != "custom-model" {
		t.Errorf("cfg = %+v", cfg)
	}
	if cfg.Memory.Embedding.Provider != "ollama" || cfg.Memory.Embedding.Model != "nomic-embed-text" {
		t.Errorf("embedding = %+v", cfg.Memory.Embedding)
	}
}
