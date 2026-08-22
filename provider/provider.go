package provider

import "context"

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
