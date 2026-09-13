package openrouter

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEmbedSendsRequestAndMapsVectors(t *testing.T) {
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
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("request body: %v\n%s", err, raw)
		}
		if body.Model != "openai/text-embedding-3-small" {
			t.Errorf("model = %q, want openai/text-embedding-3-small", body.Model)
		}
		if len(body.Input) != 2 || body.Input[0] != "hello" || body.Input[1] != "world" {
			t.Errorf("input = %v, want hello, world", body.Input)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": []any{
				map[string]any{"index": 1, "embedding": []float32{0.5, 0.75}},
				map[string]any{"index": 0, "embedding": []float32{0.25, 0.5}},
			},
		})
	}))
	defer ts.Close()

	e := NewEmbedder("test-key", "openai/text-embedding-3-small")
	e.url = ts.URL

	got, err := e.Embed(context.Background(), []string{"hello", "world"})
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

func TestEmbedAPIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"message": "nope"},
		})
	}))
	defer ts.Close()

	e := NewEmbedder("test-key", "openai/text-embedding-3-small")
	e.url = ts.URL
	_, err := e.Embed(context.Background(), []string{"hello"})
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("err = %v, want nope", err)
	}
}
