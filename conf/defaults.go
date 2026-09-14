package conf

func defaults() Conf {
	return Conf{
		DefaultModel: ModelConfig{Provider: "openrouter", Model: "openai/gpt-4o-mini"},
		FastModel:    ModelConfig{Provider: "openrouter", Model: "openai/gpt-4o-mini"},
		Memory: &MemoryConfig{
			Embedding: ModelConfig{
				Provider: "openrouter",
				Model:    "openai/text-embedding-3-small",
			},
			BudgetTokens:  800,
			MinSimilarity: 0.5,
		},
	}
}
