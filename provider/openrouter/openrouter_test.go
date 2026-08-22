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
			Model    string             `json:"model"`
			Messages []provider.Message `json:"messages"`
			Tools    []struct {
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
