package remote

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/terracotta4u/golem/provider"
)

func TestChatSendsRequestAndMapsToolCall(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/chat" {
			t.Errorf("path = %s, want /v1/chat", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("Authorization = %q, want Bearer secret", got)
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
				Name        string         `json:"name"`
				Description string         `json:"description"`
				Parameters  map[string]any `json:"parameters"`
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
		if len(body.Tools) != 1 || body.Tools[0].Name != "read" {
			t.Errorf("tools = %+v, want one read tool", body.Tools)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(provider.Message{
			Role: "assistant",
			ToolCalls: []provider.ToolCall{{
				ID:   "call_1",
				Type: "function",
				Function: provider.FunctionCall{
					Name:      "read",
					Arguments: `{"path":"foo.go"}`,
				},
			}},
		})
	}))
	defer ts.Close()

	msg, err := New(ts.URL, "secret").ForModel("openai/gpt-4o-mini").Chat(context.Background(), provider.ChatRequest{
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

func TestChatStructuredSendsSchema(t *testing.T) {
	schema := provider.JSONSchema{
		Name:   "memories",
		Strict: true,
		Schema: map[string]any{"type": "object"},
	}
	want := `{"memories":["User prefers uv."]}`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/structured" {
			t.Errorf("path = %s, want /v1/chat/structured", r.URL.Path)
		}
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		var body struct {
			Model    string             `json:"model"`
			Messages []provider.Message `json:"messages"`
			Schema   struct {
				Name   string         `json:"name"`
				Strict bool           `json:"strict"`
				Schema map[string]any `json:"schema"`
			} `json:"schema"`
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("request body: %v\n%s", err, raw)
		}
		if body.Model != "openai/gpt-4o-mini" {
			t.Errorf("model = %q", body.Model)
		}
		if len(body.Messages) != 1 || body.Messages[0].Content != "extract" {
			t.Errorf("messages = %+v", body.Messages)
		}
		if body.Schema.Name != schema.Name || body.Schema.Strict != schema.Strict || body.Schema.Schema["type"] != "object" {
			t.Errorf("schema = %+v", body.Schema)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":` + want + `}`))
	}))
	defer ts.Close()

	got, err := New(ts.URL, "secret").ForModel("openai/gpt-4o-mini").ChatStructured(context.Background(), []provider.Message{
		{Role: "user", Content: "extract"},
	}, schema)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("data = %s, want %s", got, want)
	}
}

func TestChatStructuredUnsupportedFormat(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(`{"error":{"code":"unsupported_format","message":"no json schema"}}`))
	}))
	defer ts.Close()

	_, err := New(ts.URL, "secret").ForModel("m").ChatStructured(context.Background(), []provider.Message{
		{Role: "user", Content: "extract"},
	}, provider.JSONSchema{Name: "memories"})
	if !errors.Is(err, provider.ErrUnsupportedFormat) {
		t.Fatalf("err = %v, want ErrUnsupportedFormat", err)
	}
}

func TestChatNon200(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"code":"bad_request","message":"nope"}}`))
	}))
	defer ts.Close()

	_, err := New(ts.URL, "secret").ForModel("m").Chat(context.Background(), provider.ChatRequest{})
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("err = %v, want nope", err)
	}
	if errors.Is(err, provider.ErrUnsupportedFormat) {
		t.Fatal("non-unsupported error should not match ErrUnsupportedFormat")
	}
}

func TestEmbedSendsRequestAndMapsVectors(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embed" {
			t.Errorf("path = %s, want /v1/embed", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("Authorization = %q, want Bearer secret", got)
		}
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		var body struct {
			Model string   `json:"model"`
			Texts []string `json:"texts"`
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("request body: %v\n%s", err, raw)
		}
		if body.Model != "openai/text-embedding-3-small" {
			t.Errorf("model = %q", body.Model)
		}
		if len(body.Texts) != 2 || body.Texts[0] != "hello" || body.Texts[1] != "world" {
			t.Errorf("texts = %v, want hello, world", body.Texts)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"vectors": [][]float32{{0.25, 0.5}, {0.5, 0.75}},
		})
	}))
	defer ts.Close()

	got, err := New(ts.URL, "secret").ForModel("openai/text-embedding-3-small").Embed(context.Background(), []string{"hello", "world"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if len(got[0]) != 2 || got[0][0] != 0.25 || got[0][1] != 0.5 {
		t.Errorf("vec 0 = %v, want [0.25 0.5]", got[0])
	}
	if len(got[1]) != 2 || got[1][0] != 0.5 || got[1][1] != 0.75 {
		t.Errorf("vec 1 = %v, want [0.5 0.75]", got[1])
	}
}

func TestEmbedNon200(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`{"error":{"message":"down"}}`))
	}))
	defer ts.Close()

	_, err := New(ts.URL, "secret").ForModel("m").Embed(context.Background(), []string{"hello"})
	if err == nil || !strings.Contains(err.Error(), "down") {
		t.Fatalf("err = %v, want down", err)
	}
}
