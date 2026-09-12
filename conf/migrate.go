package conf

// migrate fills fields that are missing from an older conf.json.
// Current defaults() is the source of truth; add new keys here as conf grows.
func migrate(cfg *Conf) {
	d := defaults()
	if cfg.Provider == "" {
		cfg.Provider = d.Provider
	}
	if cfg.Model == "" {
		cfg.Model = d.Model
	}
	if cfg.Memory == nil {
		cfg.Memory = d.Memory
		return
	}
	if cfg.Memory.Embedding.Provider == "" {
		cfg.Memory.Embedding.Provider = d.Memory.Embedding.Provider
	}
	if cfg.Memory.Embedding.Model == "" {
		cfg.Memory.Embedding.Model = d.Memory.Embedding.Model
	}
}
