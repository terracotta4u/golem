package provider

import (
	"context"
	"encoding/json"
	"fmt"
)

type chatModeler interface {
	ForModel(model string) Provider
}

type embedModeler interface {
	ForModel(model string) Embedder
}

type modelChat struct {
	fn func(model string) Provider
}

func NewModelChat(fn func(model string) Provider) Provider {
	return modelChat{fn}
}

func (m modelChat) ForModel(model string) Provider {
	return m.fn(model)
}

func (m modelChat) Chat(ctx context.Context, req ChatRequest) (Message, error) {
	return m.fn("").Chat(ctx, req)
}

type modelEmbedder struct {
	fn func(model string) Embedder
}

func NewModelEmbedder(fn func(model string) Embedder) Embedder {
	return modelEmbedder{fn}
}

func (m modelEmbedder) ForModel(model string) Embedder {
	return m.fn(model)
}

func (m modelEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	return m.fn("").Embed(ctx, texts)
}

type lazyChat struct {
	hub    *Hub
	config func() (name, model string, err error)
}

func NewLazyChat(hub *Hub, config func() (name, model string, err error)) Provider {
	return lazyChat{hub: hub, config: config}
}

func (l lazyChat) Chat(ctx context.Context, req ChatRequest) (Message, error) {
	p, err := l.resolve()
	if err != nil {
		return Message{}, err
	}
	return p.Chat(ctx, req)
}

func (l lazyChat) ChatStructured(ctx context.Context, msgs []Message, schema JSONSchema) (json.RawMessage, error) {
	p, err := l.resolve()
	if err != nil {
		return nil, err
	}
	s, ok := p.(Structured)
	if !ok {
		return nil, ErrUnsupportedFormat
	}
	return s.ChatStructured(ctx, msgs, schema)
}

func (l lazyChat) resolve() (Provider, error) {
	name, model, err := l.config()
	if err != nil {
		return nil, err
	}
	b, err := l.hub.Get(name)
	if err != nil {
		return nil, err
	}
	if b.Chat == nil {
		return nil, fmt.Errorf("provider %q does not support chat", name)
	}
	if m, ok := b.Chat.(chatModeler); ok {
		return m.ForModel(model), nil
	}
	return b.Chat, nil
}

type lazyEmbedder struct {
	hub    *Hub
	config func() (name, model string, err error)
}

func NewLazyEmbedder(hub *Hub, config func() (name, model string, err error)) Embedder {
	return lazyEmbedder{hub: hub, config: config}
}

func (l lazyEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	name, model, err := l.config()
	if err != nil {
		return nil, err
	}
	b, err := l.hub.Get(name)
	if err != nil {
		return nil, err
	}
	if b.Embedder == nil {
		return nil, fmt.Errorf("provider %q does not support embedding", name)
	}
	e := b.Embedder
	if m, ok := e.(embedModeler); ok {
		e = m.ForModel(model)
	}
	return e.Embed(ctx, texts)
}
