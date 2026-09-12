package conf

func defaults() Conf {
	return Conf{
		Provider: "openrouter",
		Model:    "openai/gpt-4o-mini",
		Memory: &MemoryConfig{
			Embedding: EmbeddingConfig{
				Provider: "openrouter",
				Model:    "openai/text-embedding-3-small",
			},
		},
	}
}
