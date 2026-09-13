package provider

import "fmt"

type EmbedderFactory func(model string) (Embedder, error)

type Registry struct {
	embedders map[string]EmbedderFactory
}

func NewRegistry() *Registry {
	return &Registry{embedders: make(map[string]EmbedderFactory)}
}

func (r *Registry) RegisterEmbedder(name string, factory EmbedderFactory) {
	r.embedders[name] = factory
}

func (r *Registry) Embedder(name, model string) (Embedder, error) {
	factory, ok := r.embedders[name]
	if !ok {
		return nil, fmt.Errorf("unknown embedding provider %q", name)
	}
	return factory(model)
}
