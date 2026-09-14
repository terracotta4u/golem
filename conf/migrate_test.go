package conf

import "testing"

func TestMigrateFillsMissingFields(t *testing.T) {
	cfg := Conf{DefaultModel: ModelConfig{Model: "custom-model"}}
	migrate(&cfg)

	d := defaults()
	if cfg.DefaultModel.Provider != d.DefaultModel.Provider {
		t.Errorf("default provider = %q, want %q", cfg.DefaultModel.Provider, d.DefaultModel.Provider)
	}
	if cfg.DefaultModel.Model != "custom-model" {
		t.Errorf("default model = %q, want custom-model", cfg.DefaultModel.Model)
	}
	if cfg.FastModel != d.FastModel {
		t.Errorf("fast = %+v, want %+v", cfg.FastModel, d.FastModel)
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

func TestMigrateReportsWhetherConfChanged(t *testing.T) {
	cfg := Conf{DefaultModel: ModelConfig{Model: "custom-model"}}
	if !migrate(&cfg) {
		t.Fatal("missing fields should count as a change")
	}
	if migrate(&cfg) {
		t.Fatal("second migrate should not change a complete conf")
	}
}

func TestMigrateLeavesExplicitValues(t *testing.T) {
	cfg := Conf{
		DefaultModel: ModelConfig{Provider: "other", Model: "custom-model"},
		FastModel:    ModelConfig{Provider: "ollama", Model: "llama3.2"},
		Memory: &MemoryConfig{
			Embedding:     ModelConfig{Provider: "ollama", Model: "nomic-embed-text"},
			BudgetTokens:  1000,
			MinSimilarity: 0.7,
		},
	}
	migrate(&cfg)
	if cfg.DefaultModel.Provider != "other" || cfg.DefaultModel.Model != "custom-model" {
		t.Errorf("default = %+v", cfg.DefaultModel)
	}
	if cfg.FastModel.Provider != "ollama" || cfg.FastModel.Model != "llama3.2" {
		t.Errorf("fast = %+v", cfg.FastModel)
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
			Embedding: ModelConfig{Provider: "ollama", Model: "nomic-embed-text"},
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
