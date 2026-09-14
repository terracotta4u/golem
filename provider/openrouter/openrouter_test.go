package openrouter

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/terracotta4u/golem/provider"
)

func TestChatSendsRequestAndMapsToolCall(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q, want Bearer test-key", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", got)
		}

		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}

		var body struct {
			Model          string             `json:"model"`
			Messages       []provider.Message `json:"messages"`
			ResponseFormat json.RawMessage    `json:"response_format"`
			Tools          []struct {
				Type     string `json:"type"`
				Function struct {
					Name        string         `json:"name"`
					Description string         `json:"description"`
					Parameters  map[string]any `json:"parameters"`
				} `json:"function"`
			} `json:"tools"`
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("request body: %v\n%s", err, raw)
		}
		if body.Model != "openai/gpt-4o-mini" {
			t.Errorf("model = %q, want openai/gpt-4o-mini", body.Model)
		}
		if len(body.Messages) != 1 || body.Messages[0].Role != "user" || body.Messages[0].Content != "read foo.go" {
			t.Errorf("messages = %+v, want one user message", body.Messages)
		}
		if len(body.Tools) != 1 || body.Tools[0].Type != "function" || body.Tools[0].Function.Name != "read" {
			t.Errorf("tools = %+v, want one read function", body.Tools)
		}
		if len(body.ResponseFormat) != 0 {
			t.Errorf("Chat sent response_format: %s", body.ResponseFormat)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{
				map[string]any{
					"message": map[string]any{
						"role": "assistant",
						"tool_calls": []any{
							map[string]any{
								"id":   "call_1",
								"type": "function",
								"function": map[string]any{
									"name":      "read",
									"arguments": `{"path":"foo.go"}`,
								},
							},
						},
					},
				},
			},
		})
	}))
	defer ts.Close()

	c := New("test-key", "openai/gpt-4o-mini")
	c.url = ts.URL

	msg, err := c.Chat(context.Background(), provider.ChatRequest{
		Messages: []provider.Message{{Role: "user", Content: "read foo.go"}},
		Tools: []provider.ToolDef{{
			Name:        "read",
			Description: "read a file",
			Parameters:  map[string]any{"type": "object"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if msg.Role != "assistant" {
		t.Errorf("role = %q, want assistant", msg.Role)
	}
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("tool calls = %d, want 1", len(msg.ToolCalls))
	}
	call := msg.ToolCalls[0]
	if call.ID != "call_1" || call.Function.Name != "read" || call.Function.Arguments != `{"path":"foo.go"}` {
		t.Errorf("tool call = %+v", call)
	}
}

func TestChatStructuredSendsJSONSchema(t *testing.T) {
	schema := provider.JSONSchema{
		Name:   "memories",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"memories": map[string]any{"type": "array"},
			},
		},
	}
	wantContent := `{"memories":["User prefers using uv for projects."]}`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		var body struct {
			Model          string             `json:"model"`
			Messages       []provider.Message `json:"messages"`
			Tools          json.RawMessage    `json:"tools"`
			ResponseFormat struct {
				Type       string `json:"type"`
				JSONSchema struct {
					Name   string         `json:"name"`
					Strict bool           `json:"strict"`
					Schema map[string]any `json:"schema"`
				} `json:"json_schema"`
			} `json:"response_format"`
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("request body: %v\n%s", err, raw)
		}
		if body.Model != "openai/gpt-4o-mini" {
			t.Errorf("model = %q", body.Model)
		}
		if len(body.Messages) != 1 || body.Messages[0].Role != "user" || body.Messages[0].Content != "extract" {
			t.Errorf("messages = %+v", body.Messages)
		}
		if len(body.Tools) != 0 {
			t.Errorf("tools = %s, want none", body.Tools)
		}
		if body.ResponseFormat.Type != "json_schema" {
			t.Errorf("response_format.type = %q", body.ResponseFormat.Type)
		}
		got := body.ResponseFormat.JSONSchema
		if got.Name != schema.Name || got.Strict != schema.Strict {
			t.Errorf("json_schema name/strict = %q %v", got.Name, got.Strict)
		}
		if got.Schema["type"] != "object" {
			t.Errorf("schema = %+v", got.Schema)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{
				map[string]any{
					"message": map[string]any{
						"role":    "assistant",
						"content": wantContent,
					},
				},
			},
		})
	}))
	defer ts.Close()

	c := New("test-key", "openai/gpt-4o-mini")
	c.url = ts.URL

	got, err := c.ChatStructured(context.Background(), []provider.Message{
		{Role: "user", Content: "extract"},
	}, schema)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != wantContent {
		t.Errorf("content = %s, want %s", got, wantContent)
	}
}
