package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/terracotta4u/golem/provider"
	"github.com/terracotta4u/golem/registry"
	"github.com/terracotta4u/golem/server"
)

func TestRegisterProviderAndChat(t *testing.T) {
	cb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("Authorization = %q, want Bearer secret", got)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/chat":
			json.NewEncoder(w).Encode(provider.Message{Role: "assistant", Content: "hi"})
		case "/v1/embed":
			json.NewEncoder(w).Encode(map[string]any{"vectors": [][]float32{{0.1}}})
		default:
			t.Errorf("path = %s", r.URL.Path)
		}
	}))
	defer cb.Close()

	reg := registry.New("secret")
	s := server.New(server.Options{Token: "secret", Registry: reg})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	registerExt(t, ts.URL, "secret", map[string]any{
		"name":         "golem-openrouter",
		"callback_url": cb.URL,
		"providers":    []any{map[string]any{"id": "openrouter", "chat": true, "structured": true, "embed": true}},
	})

	list := listExts(t, ts.URL, "secret")
	if len(list) != 1 || list[0].Name != "golem-openrouter" || list[0].CallbackURL != cb.URL {
		t.Fatalf("list = %+v", list)
	}
	if len(list[0].Providers) != 1 || list[0].Providers[0].ID != "openrouter" || !list[0].Providers[0].Chat {
		t.Fatalf("providers = %+v", list[0].Providers)
	}

	msg, err := chatProvider(reg, "openrouter").Chat(context.Background(), provider.ChatRequest{
		Messages: []provider.Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if msg.Content != "hi" {
		t.Errorf("content = %q, want hi", msg.Content)
	}
	vecs, err := embedProvider(reg, "openrouter").Embed(context.Background(), []string{"hello"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vecs) != 1 || vecs[0][0] != 0.1 {
		t.Fatalf("vecs = %v", vecs)
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

	reg := registry.New("secret")
	s := server.New(server.Options{Token: "secret", Registry: reg})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	registerExt(t, ts.URL, "secret", map[string]any{
		"name":         "golem-embed",
		"callback_url": cb.URL,
		"providers":    []any{map[string]any{"id": "local-embed", "embed": true}},
	})

	if _, err := chatProvider(reg, "local-embed").Chat(context.Background(), provider.ChatRequest{}); err == nil || err.Error() != `provider "local-embed" does not support chat` {
		t.Fatalf("chat err = %v", err)
	}
	vecs, err := embedProvider(reg, "local-embed").Embed(context.Background(), []string{"hello"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vecs) != 1 || vecs[0][0] != 0.1 {
		t.Fatalf("vecs = %v", vecs)
	}
}

func TestRegisterProviderRequiresRoute(t *testing.T) {
	reg := registry.New("secret")
	s := server.New(server.Options{Token: "secret", Registry: reg})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	status, raw := postJSON(t, ts.URL+"/v1/extensions/register", "secret", map[string]any{
		"name":         "ext",
		"callback_url": "http://127.0.0.1:9",
		"providers":    []any{map[string]any{"id": "p"}},
	})
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d (%s), want 400", status, raw)
	}
	if !bytes.Contains([]byte(raw), []byte("chat or embed")) {
		t.Fatalf("body = %s, want chat or embed", raw)
	}
	if err := resolveProvider(reg, "p"); err == nil || err.Error() != `unknown provider "p"` {
		t.Fatalf("err = %v, want unknown provider", err)
	}
}

func TestRegisterToolListed(t *testing.T) {
	reg := registry.New("secret")
	s := server.New(server.Options{Token: "secret", Registry: reg})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	registerExt(t, ts.URL, "secret", map[string]any{
		"name":         "golem-weather",
		"callback_url": "http://127.0.0.1:9",
		"tools": []any{map[string]any{
			"name":        "weather",
			"description": "Current conditions for a city.",
			"parameters":  map[string]any{"type": "object", "properties": map[string]any{"city": map[string]any{"type": "string"}}},
		}},
	})

	list := listExts(t, ts.URL, "secret")
	if len(list) != 1 || len(list[0].Tools) != 1 {
		t.Fatalf("list = %+v", list)
	}
	got := list[0].Tools[0]
	if got.Name != "weather" || got.Description != "Current conditions for a city." {
		t.Fatalf("tool = %+v", got)
	}
	if !bytes.Contains(got.Parameters, []byte(`"city"`)) {
		t.Fatalf("parameters = %s", got.Parameters)
	}
	if err := resolveProvider(reg, "weather"); err == nil || err.Error() != `unknown provider "weather"` {
		t.Fatalf("err = %v, want unknown provider", err)
	}
}

func TestRegisterToolRejectsBuiltin(t *testing.T) {
	s := server.New(server.Options{Token: "secret"})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	for _, name := range []string{"read", "write", "edit", "shell", "skill"} {
		status, raw := postJSON(t, ts.URL+"/v1/extensions/register", "secret", map[string]any{
			"name":         "golem-weather",
			"callback_url": "http://127.0.0.1:9",
			"tools":        []any{toolCap(name)},
		})
		if status != http.StatusConflict || !bytes.Contains([]byte(raw), []byte("builtin")) {
			t.Fatalf("tool %s status = %d (%s), want 409 builtin", name, status, raw)
		}
	}
	if len(listExts(t, ts.URL, "secret")) != 0 {
		t.Fatal("rejected tool should not be listed")
	}
}

func TestRegisterToolRejectsDuplicate(t *testing.T) {
	s := server.New(server.Options{Token: "secret"})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	registerExt(t, ts.URL, "secret", map[string]any{
		"name":         "one",
		"callback_url": "http://127.0.0.1:9",
		"tools":        []any{toolCap("weather")},
	})
	status, raw := postJSON(t, ts.URL+"/v1/extensions/register", "secret", map[string]any{
		"name":         "two",
		"callback_url": "http://127.0.0.1:10",
		"tools":        []any{toolCap("weather")},
	})
	if status != http.StatusConflict {
		t.Fatalf("status = %d (%s), want 409", status, raw)
	}
	list := listExts(t, ts.URL, "secret")
	if len(list) != 1 || list[0].Name != "one" {
		t.Fatalf("list = %+v", list)
	}
}

func TestRegisterToolReplaceSameExtension(t *testing.T) {
	s := server.New(server.Options{Token: "secret"})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	body := map[string]any{
		"name":         "golem-weather",
		"callback_url": "http://127.0.0.1:9",
		"tools":        []any{toolCap("weather")},
	}
	registerExt(t, ts.URL, "secret", body)
	body["callback_url"] = "http://127.0.0.1:10"
	registerExt(t, ts.URL, "secret", body)

	list := listExts(t, ts.URL, "secret")
	if len(list) != 1 || list[0].CallbackURL != "http://127.0.0.1:10" || list[0].Tools[0].Name != "weather" {
		t.Fatalf("list = %+v", list)
	}
}

func TestRegisterToolExpires(t *testing.T) {
	reg := registry.New("secret")
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	reg.Now = func() time.Time { return now }
	reg.TTL = time.Minute
	s := server.New(server.Options{Token: "secret", Registry: reg})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	registerExt(t, ts.URL, "secret", map[string]any{
		"name":         "one",
		"callback_url": "http://127.0.0.1:9",
		"tools":        []any{toolCap("weather")},
	})
	now = now.Add(time.Minute)
	if len(listExts(t, ts.URL, "secret")) != 0 {
		t.Fatal("want empty list after ttl")
	}
	registerExt(t, ts.URL, "secret", map[string]any{
		"name":         "two",
		"callback_url": "http://127.0.0.1:10",
		"tools":        []any{toolCap("weather")},
	})
	list := listExts(t, ts.URL, "secret")
	if len(list) != 1 || list[0].Name != "two" {
		t.Fatalf("list = %+v", list)
	}
}

func TestRegisterToolRequiresFields(t *testing.T) {
	s := server.New(server.Options{Token: "secret"})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	cases := []struct {
		cap  map[string]any
		want string
	}{
		{map[string]any{"parameters": map[string]any{"type": "object"}}, "tool name is required"},
		{map[string]any{"name": "weather"}, "tool parameters are required"},
		{map[string]any{"name": "weather", "parameters": []any{}}, "tool parameters must be an object"},
	}
	for _, tc := range cases {
		status, raw := postJSON(t, ts.URL+"/v1/extensions/register", "secret", map[string]any{
			"name":         "golem-weather",
			"callback_url": "http://127.0.0.1:9",
			"tools":        []any{tc.cap},
		})
		if status != http.StatusBadRequest || !bytes.Contains([]byte(raw), []byte(tc.want)) {
			t.Fatalf("cap %v status = %d (%s), want 400 %s", tc.cap, status, raw, tc.want)
		}
	}
}

func toolCap(name string) map[string]any {
	return map[string]any{
		"name":        name,
		"description": "A tool.",
		"parameters":  map[string]any{"type": "object", "properties": map[string]any{}},
	}
}

func TestRegisterChannelListed(t *testing.T) {
	s := server.New(server.Options{Token: "secret"})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	registerExt(t, ts.URL, "secret", map[string]any{
		"name":         "golem-cli",
		"callback_url": "http://127.0.0.1:9",
		"channels":     []any{map[string]any{"id": "cli"}},
	})

	list := listExts(t, ts.URL, "secret")
	if len(list) != 1 || list[0].Name != "golem-cli" || len(list[0].Providers) != 0 || len(list[0].Tools) != 0 || len(list[0].Channels) != 1 || list[0].Channels[0].ID != "cli" {
		t.Fatalf("list = %+v", list)
	}
}

func TestRegisterRejectsNonLoopback(t *testing.T) {
	s := server.New(server.Options{Token: "secret"})
	ts := httptest.NewServer(s.Handler())
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
			"providers":    []any{map[string]any{"id": "p", "chat": true}},
		})
		if status != http.StatusBadRequest {
			t.Errorf("callback_url %q status = %d (%s), want 400", url, status, body)
		}
	}
}

func TestRegisterDuplicateProvider(t *testing.T) {
	s := server.New(server.Options{Token: "secret"})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	body := map[string]any{
		"name":         "one",
		"callback_url": "http://127.0.0.1:9",
		"providers":    []any{map[string]any{"id": "openrouter", "chat": true}},
	}
	registerExt(t, ts.URL, "secret", body)
	body["name"] = "two"
	status, raw := postJSON(t, ts.URL+"/v1/extensions/register", "secret", body)
	if status != http.StatusConflict {
		t.Fatalf("status = %d (%s), want 409", status, raw)
	}
}

func TestRegisterReplacesSameName(t *testing.T) {
	reg := registry.New("secret")
	s := server.New(server.Options{Token: "secret", Registry: reg})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	registerExt(t, ts.URL, "secret", map[string]any{
		"name":         "golem-openrouter",
		"callback_url": "http://127.0.0.1:9",
		"providers":    []any{map[string]any{"id": "openrouter", "chat": true}},
	})
	registerExt(t, ts.URL, "secret", map[string]any{
		"name":         "golem-openrouter",
		"callback_url": "http://127.0.0.1:10",
		"providers":    []any{map[string]any{"id": "openrouter", "chat": true, "embed": true}},
	})

	list := listExts(t, ts.URL, "secret")
	if len(list) != 1 || list[0].CallbackURL != "http://127.0.0.1:10" {
		t.Fatalf("list = %+v", list)
	}
	if _, err := embedProvider(reg, "openrouter").Embed(context.Background(), []string{"hi"}); err == nil || strings.Contains(err.Error(), "does not support embedding") || strings.Contains(err.Error(), "unknown provider") {
		t.Fatalf("err = %v, want the embed route", err)
	}
}

func TestHeartbeatExpires(t *testing.T) {
	reg := registry.New("secret")
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	reg.Now = func() time.Time { return now }
	reg.TTL = time.Minute
	s := server.New(server.Options{Token: "secret", Registry: reg})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	registerExt(t, ts.URL, "secret", map[string]any{
		"name":         "golem-openrouter",
		"callback_url": "http://127.0.0.1:9",
		"providers":    []any{map[string]any{"id": "openrouter", "chat": true}},
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
	if err := resolveProvider(reg, "openrouter"); err == nil || err.Error() != `unknown provider "openrouter"` {
		t.Fatalf("err = %v, want unknown provider", err)
	}
}

func TestRegisterUnauthorized(t *testing.T) {
	s := server.New(server.Options{Token: "secret"})
	ts := httptest.NewServer(s.Handler())
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

func listExts(t *testing.T, base, token string) []registry.Registration {
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
		Extensions []registry.Registration `json:"extensions"`
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

func chatProvider(reg *registry.Registry, id string) provider.Provider {
	return registry.BindChat(reg, func() (string, string, error) { return id, "m", nil })
}

func embedProvider(reg *registry.Registry, id string) provider.Embedder {
	return registry.BindEmbed(reg, func() (string, string, error) { return id, "m", nil })
}

func resolveProvider(reg *registry.Registry, id string) error {
	_, err := chatProvider(reg, id).Chat(context.Background(), provider.ChatRequest{})
	return err
}
