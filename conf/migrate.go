package conf

func migrate(cfg *Conf) bool {
	d := defaults()
	changed := false
	if cfg.DefaultModel.Provider == "" {
		cfg.DefaultModel.Provider = d.DefaultModel.Provider
		changed = true
	}
	if cfg.DefaultModel.Model == "" {
		cfg.DefaultModel.Model = d.DefaultModel.Model
		changed = true
	}
	if cfg.FastModel.Provider == "" {
		cfg.FastModel.Provider = d.FastModel.Provider
		changed = true
	}
	if cfg.FastModel.Model == "" {
		cfg.FastModel.Model = d.FastModel.Model
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
