package provider

import (
	"context"
	"encoding/json"
	"errors"
)

var ErrUnsupportedFormat = errors.New("structured output not supported")

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolDef struct {
	Name        string
	Description string
	Parameters  map[string]any
}

type ChatRequest struct {
	Messages []Message
	Tools    []ToolDef
}

type Provider interface {
	Chat(ctx context.Context, req ChatRequest) (Message, error)
}

// Structured is optional. Extract uses it when the provider implements it.
type Structured interface {
	ChatStructured(ctx context.Context, msgs []Message, schema JSONSchema) (json.RawMessage, error)
}

type JSONSchema struct {
	Name   string
	Strict bool
	Schema map[string]any
}

type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}
