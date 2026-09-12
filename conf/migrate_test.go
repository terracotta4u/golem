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
	if cfg.Memory.BudgetTokens != 800 || cfg.Memory.MinSimilarity != 0.5 {
		t.Errorf("budget/min = %d/%v", cfg.Memory.BudgetTokens, cfg.Memory.MinSimilarity)
	}
}

func TestMigrateLeavesExplicitValues(t *testing.T) {
	cfg := Conf{
		Provider: "other",
		Model:    "custom-model",
		Memory: &MemoryConfig{
			Embedding:     EmbeddingConfig{Provider: "ollama", Model: "nomic-embed-text"},
			BudgetTokens:  1000,
			MinSimilarity: 0.7,
		},
	}
	migrate(&cfg)
	if cfg.Provider != "other" || cfg.Model != "custom-model" {
		t.Errorf("cfg = %+v", cfg)
	}
	if cfg.Memory.Embedding.Provider != "ollama" || cfg.Memory.Embedding.Model != "nomic-embed-text" {
		t.Errorf("embedding = %+v", cfg.Memory.Embedding)
	}
	if cfg.Memory.BudgetTokens != 1000 || cfg.Memory.MinSimilarity != 0.7 {
		t.Errorf("budget/min = %d/%v", cfg.Memory.BudgetTokens, cfg.Memory.MinSimilarity)
	}
}

func TestMigrateFillsRetrievalKnobsOnExistingMemory(t *testing.T) {
	cfg := Conf{
		Memory: &MemoryConfig{
			Embedding: EmbeddingConfig{Provider: "ollama", Model: "nomic-embed-text"},
		},
	}
	migrate(&cfg)
	if cfg.Memory.Embedding.Provider != "ollama" {
		t.Errorf("wiped embedding provider: %+v", cfg.Memory.Embedding)
	}
	if cfg.Memory.BudgetTokens != 800 || cfg.Memory.MinSimilarity != 0.5 {
		t.Errorf("budget/min = %d/%v", cfg.Memory.BudgetTokens, cfg.Memory.MinSimilarity)
	}
}
