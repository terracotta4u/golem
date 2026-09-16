package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/terracotta4u/golem/agent"
	"github.com/terracotta4u/golem/conf"
	"github.com/terracotta4u/golem/provider"
	"github.com/terracotta4u/golem/store"
)

func TestRegisterProviderAndChat(t *testing.T) {
	cb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat" {
			t.Errorf("path = %s, want /v1/chat", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("Authorization = %q, want Bearer secret", got)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(provider.Message{Role: "assistant", Content: "hi"})
	}))
	defer cb.Close()

	s := New(Options{Token: "secret"})
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	registerExt(t, ts.URL, "secret", map[string]any{
		"name":         "golem-openrouter",
		"callback_url": cb.URL,
		"capabilities": []any{map[string]any{"kind": "provider", "id": "openrouter", "chat": true, "structured": true, "embed": true}},
	})

	list := listExts(t, ts.URL, "secret")
	if len(list) != 1 || list[0].Name != "golem-openrouter" || list[0].CallbackURL != cb.URL {
		t.Fatalf("list = %+v", list)
	}
	if len(list[0].Capabilities) != 1 || list[0].Capabilities[0].Kind != "provider" || list[0].Capabilities[0].ID != "openrouter" {
		t.Fatalf("capabilities = %+v", list[0].Capabilities)
	}

	b, err := s.hub.Get("openrouter")
	if err != nil {
		t.Fatal(err)
	}
	if b.Chat == nil || b.Embedder == nil {
		t.Fatalf("backend = %+v, want chat and embedder", b)
	}
	msg, err := b.Chat.Chat(context.Background(), provider.ChatRequest{
		Messages: []provider.Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if msg.Content != "hi" {
		t.Errorf("content = %q, want hi", msg.Content)
	}
}

func TestRegisterEmbedOnly(t *testing.T) {
	cb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embed" {
			t.Errorf("path = %s, want /v1/embed", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"vectors": [][]float32{{0.1, 0.2}}})
	}))
	defer cb.Close()

	s := New(Options{Token: "secret"})
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	registerExt(t, ts.URL, "secret", map[string]any{
		"name":         "golem-embed",
		"callback_url": cb.URL,
		"capabilities": []any{map[string]any{"kind": "provider", "id": "local-embed", "embed": true}},
	})

	b, err := s.hub.Get("local-embed")
	if err != nil {
		t.Fatal(err)
	}
	if b.Chat != nil {
		t.Fatal("embed-only provider should not register chat")
	}
	if b.Embedder == nil {
		t.Fatal("embed-only provider missing embedder")
	}
	vecs, err := b.Embedder.Embed(context.Background(), []string{"hello"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vecs) != 1 || vecs[0][0] != 0.1 {
		t.Fatalf("vecs = %v", vecs)
	}
}

func TestRegisterProviderRequiresRoute(t *testing.T) {
	s := New(Options{Token: "secret"})
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	status, raw := postJSON(t, ts.URL+"/v1/extensions/register", "secret", map[string]any{
		"name":         "ext",
		"callback_url": "http://127.0.0.1:9",
		"capabilities": []any{map[string]any{"kind": "provider", "id": "p"}},
	})
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d (%s), want 400", status, raw)
	}
	if !bytes.Contains([]byte(raw), []byte("chat or embed")) {
		t.Fatalf("body = %s, want chat or embed", raw)
	}
	if _, err := s.hub.Get("p"); err == nil {
		t.Fatal("empty provider should not register")
	}
}

func TestRegisterUnknownKindListed(t *testing.T) {
	s := New(Options{Token: "secret"})
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	registerExt(t, ts.URL, "secret", map[string]any{
		"name":         "custom",
		"callback_url": "http://127.0.0.1:9",
		"capabilities": []any{map[string]any{"kind": "widget", "id": "w1"}},
	})

	list := listExts(t, ts.URL, "secret")
	if len(list) != 1 || list[0].Name != "custom" || len(list[0].Capabilities) != 1 || list[0].Capabilities[0].Kind != "widget" {
		t.Fatalf("list = %+v", list)
	}
	if _, err := s.hub.Get("w1"); err == nil {
		t.Fatal("unknown kind should not register a provider")
	}
}

func TestRegisterRejectsNonLoopback(t *testing.T) {
	s := New(Options{Token: "secret"})
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	for _, url := range []string{
		"http://example.com",
		"https://127.0.0.1",
		"http://10.0.0.1:9",
		"http://0.0.0.0:9",
		"not-a-url",
	} {
		status, body := postJSON(t, ts.URL+"/v1/extensions/register", "secret", map[string]any{
			"name":         "ext",
			"callback_url": url,
			"capabilities": []any{map[string]any{"kind": "provider", "id": "p", "chat": true}},
		})
		if status != http.StatusBadRequest {
			t.Errorf("callback_url %q status = %d (%s), want 400", url, status, body)
		}
	}
}

func TestRegisterDuplicateProvider(t *testing.T) {
	s := New(Options{Token: "secret"})
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	body := map[string]any{
		"name":         "one",
		"callback_url": "http://127.0.0.1:9",
		"capabilities": []any{map[string]any{"kind": "provider", "id": "openrouter", "chat": true}},
	}
	registerExt(t, ts.URL, "secret", body)
	body["name"] = "two"
	status, raw := postJSON(t, ts.URL+"/v1/extensions/register", "secret", body)
	if status != http.StatusConflict {
		t.Fatalf("status = %d (%s), want 409", status, raw)
	}
}

func TestRegisterReplacesSameName(t *testing.T) {
	s := New(Options{Token: "secret"})
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	registerExt(t, ts.URL, "secret", map[string]any{
		"name":         "golem-openrouter",
		"callback_url": "http://127.0.0.1:9",
		"capabilities": []any{map[string]any{"kind": "provider", "id": "openrouter", "chat": true}},
	})
	registerExt(t, ts.URL, "secret", map[string]any{
		"name":         "golem-openrouter",
		"callback_url": "http://127.0.0.1:10",
		"capabilities": []any{map[string]any{"kind": "provider", "id": "openrouter", "chat": true, "embed": true}},
	})

	list := listExts(t, ts.URL, "secret")
	if len(list) != 1 || list[0].CallbackURL != "http://127.0.0.1:10" {
		t.Fatalf("list = %+v", list)
	}
	b, err := s.hub.Get("openrouter")
	if err != nil {
		t.Fatal(err)
	}
	if b.Embedder == nil {
		t.Fatal("replaced backend missing embedder")
	}
}

func TestHeartbeatExpires(t *testing.T) {
	s := New(Options{Token: "secret"})
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	s.ttl = time.Minute
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	registerExt(t, ts.URL, "secret", map[string]any{
		"name":         "golem-openrouter",
		"callback_url": "http://127.0.0.1:9",
		"capabilities": []any{map[string]any{"kind": "provider", "id": "openrouter", "chat": true}},
	})

	now = now.Add(30 * time.Second)
	status, raw := postJSON(t, ts.URL+"/v1/extensions/heartbeat", "secret", map[string]any{"name": "golem-openrouter"})
	if status != http.StatusOK {
		t.Fatalf("heartbeat status = %d (%s)", status, raw)
	}

	now = now.Add(time.Minute)
	if len(listExts(t, ts.URL, "secret")) != 0 {
		t.Fatal("want empty list after ttl")
	}
	if _, err := s.hub.Get("openrouter"); err == nil {
		t.Fatal("want unknown provider after ttl")
	}
}

func TestStopExtensionUnregisters(t *testing.T) {
	s := New(Options{Token: "secret"})
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	registerExt(t, ts.URL, "secret", map[string]any{
		"name":         "golem-openrouter",
		"callback_url": "http://127.0.0.1:9",
		"capabilities": []any{map[string]any{"kind": "provider", "id": "openrouter", "chat": true}},
	})
	if err := s.stopExtension("golem-openrouter"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.hub.Get("openrouter"); err == nil {
		t.Fatal("want unknown provider after stop")
	}
}

func TestRegisteredProviderServesTurn(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg, _, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.DefaultModel.Provider = "stub"
	cfg.DefaultModel.Model = "stub-model"
	if err := conf.Save(cfg); err != nil {
		t.Fatal(err)
	}

	cb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		var body struct {
			Model string `json:"model"`
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatal(err)
		}
		if body.Model != "stub-model" {
			t.Errorf("model = %q, want stub-model", body.Model)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(provider.Message{Role: "assistant", Content: "from stub"})
	}))
	defer cb.Close()

	hub := provider.NewHub(0)
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := New(Options{
		Agent: agent.New(provider.NewLazyChat(hub, func() (string, string, error) {
			c, _, err := conf.Load()
			if err != nil {
				return "", "", err
			}
			return c.DefaultModel.Provider, c.DefaultModel.Model, nil
		}), t.TempDir()),
		Store: st,
		Hub:   hub,
		Token: "secret",
	})
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	registerExt(t, ts.URL, "secret", map[string]any{
		"name":         "stub",
		"callback_url": cb.URL,
		"capabilities": []any{map[string]any{"kind": "provider", "id": "stub", "chat": true}},
	})

	id := postTurn(t, ts.URL, "secret", "conv-1", "hello")
	events := getTurnEvents(t, ts.URL, "secret", id)
	if len(events) != 1 || events[0].Event != "done" {
		t.Fatalf("events = %+v, want one done", events)
	}
	if got := events[0].text(); got != "from stub" {
		t.Fatalf("text = %q, want from stub", got)
	}
}

func TestRegisterUnauthorized(t *testing.T) {
	s := New(Options{Token: "secret"})
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	status, _ := postJSON(t, ts.URL+"/v1/extensions/register", "", map[string]any{
		"name":         "ext",
		"callback_url": "http://127.0.0.1:9",
	})
	if status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", status)
	}
}

func registerExt(t *testing.T, base, token string, body map[string]any) {
	t.Helper()
	status, raw := postJSON(t, base+"/v1/extensions/register", token, body)
	if status != http.StatusOK {
		t.Fatalf("register status = %d: %s", status, raw)
	}
}

func listExts(t *testing.T, base, token string) []extJSON {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/v1/extensions", nil)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d: %s", resp.StatusCode, raw)
	}
	var out struct {
		Extensions []extJSON `json:"extensions"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out.Extensions
}

func postJSON(t *testing.T, url, token string, body map[string]any) (int, string) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, string(b)
}
