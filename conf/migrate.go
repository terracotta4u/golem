package conf

func migrate(cfg *Conf) bool {
	d := defaults()
	changed := false
	if cfg.Provider == "" {
		cfg.Provider = d.Provider
		changed = true
	}
	if cfg.Model == "" {
		cfg.Model = d.Model
		changed = true
	}
	if cfg.Memory == nil {
		cfg.Memory = d.Memory
		return true
	}
	if cfg.Memory.Embedding.Provider == "" {
		cfg.Memory.Embedding.Provider = d.Memory.Embedding.Provider
		changed = true
	}
	if cfg.Memory.Embedding.Model == "" {
		cfg.Memory.Embedding.Model = d.Memory.Embedding.Model
		changed = true
	}
	if cfg.Memory.BudgetTokens == 0 {
		cfg.Memory.BudgetTokens = d.Memory.BudgetTokens
		changed = true
	}
	if cfg.Memory.MinSimilarity == 0 {
		cfg.Memory.MinSimilarity = d.Memory.MinSimilarity
		changed = true
	}
	return changed
}
