package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/terracotta4u/golem/provider"
	"github.com/terracotta4u/golem/provider/remote"
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
	client, err := b.client()
	if err != nil {
		return provider.Message{}, err
	}
	return client.Chat(ctx, req)
}

func (b boundChat) ChatStructured(ctx context.Context, msgs []provider.Message, schema provider.JSONSchema) (json.RawMessage, error) {
	client, err := b.client()
	if err != nil {
		return nil, err
	}
	return client.ChatStructured(ctx, msgs, schema)
}

func (b boundChat) client() (*remote.Bound, error) {
	id, model, err := b.config()
	if err != nil {
		return nil, err
	}
	id = strings.TrimSpace(id)
	p, err := b.reg.provider(id)
	if err != nil {
		return nil, err
	}
	if !p.supportsChat() {
		return nil, fmt.Errorf("provider %q does not support chat", id)
	}
	return p.client.ForModel(model), nil
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
