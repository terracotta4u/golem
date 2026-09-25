package server

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

	"github.com/terracotta4u/golem/agent"
	"github.com/terracotta4u/golem/conf"
	"github.com/terracotta4u/golem/conversation"
	"github.com/terracotta4u/golem/provider"
	"github.com/terracotta4u/golem/registry"
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

	msg, err := chatProvider(s, "openrouter").Chat(context.Background(), provider.ChatRequest{
		Messages: []provider.Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if msg.Content != "hi" {
		t.Errorf("content = %q, want hi", msg.Content)
	}
	vecs, err := embedProvider(s, "openrouter").Embed(context.Background(), []string{"hello"})
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

	s := New(Options{Token: "secret"})
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	registerExt(t, ts.URL, "secret", map[string]any{
		"name":         "golem-embed",
		"callback_url": cb.URL,
		"capabilities": []any{map[string]any{"kind": "provider", "id": "local-embed", "embed": true}},
	})

	if _, err := chatProvider(s, "local-embed").Chat(context.Background(), provider.ChatRequest{}); err == nil || err.Error() != `provider "local-embed" does not support chat` {
		t.Fatalf("chat err = %v", err)
	}
	vecs, err := embedProvider(s, "local-embed").Embed(context.Background(), []string{"hello"})
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
	if err := resolveProvider(s, "p"); err == nil || err.Error() != `unknown provider "p"` {
		t.Fatalf("err = %v, want unknown provider", err)
	}
}

func TestRegisterToolListed(t *testing.T) {
	s := New(Options{Token: "secret"})
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	registerExt(t, ts.URL, "secret", map[string]any{
		"name":         "golem-weather",
		"callback_url": "http://127.0.0.1:9",
		"capabilities": []any{map[string]any{
			"kind":        "tool",
			"name":        "weather",
			"description": "Current conditions for a city.",
			"parameters":  map[string]any{"type": "object", "properties": map[string]any{"city": map[string]any{"type": "string"}}},
		}},
	})

	list := listExts(t, ts.URL, "secret")
	if len(list) != 1 || len(list[0].Capabilities) != 1 {
		t.Fatalf("list = %+v", list)
	}
	cap := list[0].Capabilities[0]
	if cap.Kind != "tool" || cap.Name != "weather" || cap.Description != "Current conditions for a city." {
		t.Fatalf("capability = %+v", cap)
	}
	if !bytes.Contains(cap.Parameters, []byte(`"city"`)) {
		t.Fatalf("parameters = %s", cap.Parameters)
	}
	if err := resolveProvider(s, "weather"); err == nil || err.Error() != `unknown provider "weather"` {
		t.Fatalf("err = %v, want unknown provider", err)
	}
}

func TestRegisteredToolIsCalled(t *testing.T) {
	var gotPath, gotAuth string
	var gotBody []byte
	cb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		var err error
		gotBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"result": "sunny in Lisbon"})
	}))
	defer cb.Close()

	p := &gatedScript{gateAt: -1, replies: []provider.Message{
		{
			Role: "assistant",
			ToolCalls: []provider.ToolCall{{
				ID:   "call_1",
				Type: "function",
				Function: provider.FunctionCall{
					Name:      "weather",
					Arguments: `{"city":"Lisbon"}`,
				},
			}},
		},
		{Role: "assistant", Content: "It is sunny."},
	}}
	st, err := conversation.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := New(Options{
		Agent: agent.New(p, t.TempDir()),
		Store: st,
		Token: "secret",
	})
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	registerExt(t, ts.URL, "secret", map[string]any{
		"name":         "golem-weather",
		"callback_url": cb.URL,
		"capabilities": []any{toolCap("weather")},
	})
	id := postTurn(t, ts.URL, "secret", "conv-1", "weather in Lisbon")
	events := getTurnEvents(t, ts.URL, "secret", id)
	if len(events) != 2 || events[0].Event != "log" || events[1].Event != "done" {
		t.Fatalf("events = %+v, want log then done", events)
	}
	if got := events[1].text(); got != "It is sunny." {
		t.Fatalf("text = %q, want It is sunny.", got)
	}
	if line := events[0].line(); !strings.Contains(line, "[weather]") {
		t.Fatalf("log line = %q", line)
	}
	if !strings.Contains(events[0].Data, "sunny in Lisbon") {
		t.Fatalf("log data = %s", events[0].Data)
	}
	if gotPath != "/v1/tools/weather" {
		t.Fatalf("path = %s", gotPath)
	}
	if gotAuth != "Bearer secret" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if string(gotBody) != `{"city":"Lisbon"}` {
		t.Fatalf("body = %s", gotBody)
	}
}

func TestRegisterToolRejectsBuiltin(t *testing.T) {
	s := New(Options{Token: "secret"})
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	for _, name := range []string{"read", "write", "edit", "shell", "skill"} {
		status, raw := postJSON(t, ts.URL+"/v1/extensions/register", "secret", map[string]any{
			"name":         "golem-weather",
			"callback_url": "http://127.0.0.1:9",
			"capabilities": []any{toolCap(name)},
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
	s := New(Options{Token: "secret"})
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	registerExt(t, ts.URL, "secret", map[string]any{
		"name":         "one",
		"callback_url": "http://127.0.0.1:9",
		"capabilities": []any{toolCap("weather")},
	})
	status, raw := postJSON(t, ts.URL+"/v1/extensions/register", "secret", map[string]any{
		"name":         "two",
		"callback_url": "http://127.0.0.1:10",
		"capabilities": []any{toolCap("weather")},
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
	s := New(Options{Token: "secret"})
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	body := map[string]any{
		"name":         "golem-weather",
		"callback_url": "http://127.0.0.1:9",
		"capabilities": []any{toolCap("weather")},
	}
	registerExt(t, ts.URL, "secret", body)
	body["callback_url"] = "http://127.0.0.1:10"
	registerExt(t, ts.URL, "secret", body)

	list := listExts(t, ts.URL, "secret")
	if len(list) != 1 || list[0].CallbackURL != "http://127.0.0.1:10" || list[0].Capabilities[0].Name != "weather" {
		t.Fatalf("list = %+v", list)
	}
}

func TestRegisterToolExpires(t *testing.T) {
	s := New(Options{Token: "secret"})
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.reg.Now = func() time.Time { return now }
	s.reg.TTL = time.Minute
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	registerExt(t, ts.URL, "secret", map[string]any{
		"name":         "one",
		"callback_url": "http://127.0.0.1:9",
		"capabilities": []any{toolCap("weather")},
	})
	now = now.Add(time.Minute)
	if len(listExts(t, ts.URL, "secret")) != 0 {
		t.Fatal("want empty list after ttl")
	}
	registerExt(t, ts.URL, "secret", map[string]any{
		"name":         "two",
		"callback_url": "http://127.0.0.1:10",
		"capabilities": []any{toolCap("weather")},
	})
	list := listExts(t, ts.URL, "secret")
	if len(list) != 1 || list[0].Name != "two" {
		t.Fatalf("list = %+v", list)
	}
}

func TestRegisterToolRequiresFields(t *testing.T) {
	s := New(Options{Token: "secret"})
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	cases := []struct {
		cap  map[string]any
		want string
	}{
		{map[string]any{"kind": "tool", "parameters": map[string]any{"type": "object"}}, "tool name is required"},
		{map[string]any{"kind": "tool", "name": "weather"}, "tool parameters are required"},
		{map[string]any{"kind": "tool", "name": "weather", "parameters": []any{}}, "tool parameters must be an object"},
	}
	for _, tc := range cases {
		status, raw := postJSON(t, ts.URL+"/v1/extensions/register", "secret", map[string]any{
			"name":         "golem-weather",
			"callback_url": "http://127.0.0.1:9",
			"capabilities": []any{tc.cap},
		})
		if status != http.StatusBadRequest || !bytes.Contains([]byte(raw), []byte(tc.want)) {
			t.Fatalf("cap %v status = %d (%s), want 400 %s", tc.cap, status, raw, tc.want)
		}
	}
}

func toolCap(name string) map[string]any {
	return map[string]any{
		"kind":        "tool",
		"name":        name,
		"description": "A tool.",
		"parameters":  map[string]any{"type": "object", "properties": map[string]any{}},
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
	if err := resolveProvider(s, "w1"); err == nil || err.Error() != `unknown provider "w1"` {
		t.Fatalf("err = %v, want unknown provider", err)
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
	if _, err := embedProvider(s, "openrouter").Embed(context.Background(), []string{"hi"}); err == nil || strings.Contains(err.Error(), "does not support embedding") || strings.Contains(err.Error(), "unknown provider") {
		t.Fatalf("err = %v, want the embed route", err)
	}
}

func TestHeartbeatExpires(t *testing.T) {
	s := New(Options{Token: "secret"})
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.reg.Now = func() time.Time { return now }
	s.reg.TTL = time.Minute
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
	if err := resolveProvider(s, "openrouter"); err == nil || err.Error() != `unknown provider "openrouter"` {
		t.Fatalf("err = %v, want unknown provider", err)
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
	if err := resolveProvider(s, "openrouter"); err == nil || err.Error() != `unknown provider "openrouter"` {
		t.Fatalf("err = %v, want unknown provider", err)
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

	reg := registry.New("")
	st, err := conversation.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := New(Options{
		Agent: agent.New(registry.BindChat(reg, func() (string, string, error) {
			c, _, err := conf.Load()
			if err != nil {
				return "", "", err
			}
			return c.DefaultModel.Provider, c.DefaultModel.Model, nil
		}), t.TempDir()),
		Store:    st,
		Registry: reg,
		Token:    "secret",
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

func chatProvider(s *Server, id string) provider.Provider {
	return registry.BindChat(s.reg, func() (string, string, error) { return id, "m", nil })
}

func embedProvider(s *Server, id string) provider.Embedder {
	return registry.BindEmbed(s.reg, func() (string, string, error) { return id, "m", nil })
}

func resolveProvider(s *Server, id string) error {
	_, err := chatProvider(s, id).Chat(context.Background(), provider.ChatRequest{})
	return err
}
