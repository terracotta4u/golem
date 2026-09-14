package provider

import "fmt"

type EmbedderFactory func(model string) (Embedder, error)

type ChatFactory func(model, apiKey string) (Provider, error)

type Registry struct {
	embedders map[string]EmbedderFactory
	chats     map[string]ChatFactory
}

func NewRegistry() *Registry {
	return &Registry{
		embedders: make(map[string]EmbedderFactory),
		chats:     make(map[string]ChatFactory),
	}
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

func (r *Registry) RegisterChat(name string, factory ChatFactory) {
	r.chats[name] = factory
}

func (r *Registry) Chat(name, model, apiKey string) (Provider, error) {
	factory, ok := r.chats[name]
	if !ok {
		return nil, fmt.Errorf("unknown chat provider %q", name)
	}
	return factory(model, apiKey)
}
