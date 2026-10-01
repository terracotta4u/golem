package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/terracotta4u/golem/provider"
)

type modelConfig func() (id, model string, err error)

type boundChat struct {
	reg    *Registry
	config modelConfig
}

type boundEmbed struct {
	reg    *Registry
	config modelConfig
}

// BindChat reads config on every call, then POSTs to the live provider with that id.
// The result implements provider.Structured.
func BindChat(reg *Registry, config func() (id, model string, err error)) provider.Provider {
	return boundChat{reg: reg, config: config}
}

// BindEmbed reads config on every call, then POSTs an embedding request to the live provider.
func BindEmbed(reg *Registry, config func() (id, model string, err error)) provider.Embedder {
	return boundEmbed{reg: reg, config: config}
}

var (
	_ provider.Provider   = boundChat{}
	_ provider.Structured = boundChat{}
	_ provider.Embedder   = boundEmbed{}
)

func (b boundChat) Chat(ctx context.Context, req provider.ChatRequest) (provider.Message, error) {
	p, model, err := b.resolve()
	if err != nil {
		return provider.Message{}, err
	}
	if !p.chat {
		return provider.Message{}, fmt.Errorf("provider %q does not support chat", p.id)
	}
	return p.client.ForModel(model).Chat(ctx, req)
}

func (b boundChat) ChatStructured(ctx context.Context, msgs []provider.Message, schema provider.JSONSchema) (json.RawMessage, error) {
	p, model, err := b.resolve()
	if err != nil {
		return nil, err
	}
	// Unadvertised structured output is a format mismatch, so callers such as
	// memory extraction can fall back to ordinary chat. A model that advertises
	// the route and still rejects a schema returns the same error from the HTTP call.
	if !p.structured {
		return nil, provider.ErrUnsupportedFormat
	}
	return p.client.ForModel(model).ChatStructured(ctx, msgs, schema)
}

func (b boundChat) resolve() (providerCap, string, error) {
	id, model, err := b.config()
	if err != nil {
		return providerCap{}, "", err
	}
	id = strings.TrimSpace(id)
	p, err := b.reg.provider(id)
	if err != nil {
		return providerCap{}, "", err
	}
	return p, model, nil
}

func (b boundEmbed) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	id, model, err := b.config()
	if err != nil {
		return nil, err
	}
	id = strings.TrimSpace(id)
	p, err := b.reg.provider(id)
	if err != nil {
		return nil, err
	}
	if !p.embed {
		return nil, fmt.Errorf("provider %q does not support embedding", id)
	}
	return p.client.ForModel(model).Embed(ctx, texts)
}
