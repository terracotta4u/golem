package registry

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/terracotta4u/golem/provider"
)

func TestBindChatSendsModel(t *testing.T) {
	var gotModel, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat" {
			t.Errorf("path = %s", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		var body struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		gotModel = body.Model
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(provider.Message{Role: "assistant", Content: "ok"})
	}))
	defer srv.Close()

	reg := New(provider.NewHub(0), "secret")
	registerProvider(t, reg, "stub", srv.URL, true, false, false)

	p := BindChat(reg, func() (string, string, error) { return "stub", "m1", nil })
	msg, err := p.Chat(context.Background(), provider.ChatRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if msg.Content != "ok" {
		t.Errorf("content = %q, want ok", msg.Content)
	}
	if gotModel != "m1" {
		t.Errorf("model = %q, want m1", gotModel)
	}
	if gotAuth != "Bearer secret" {
		t.Errorf("Authorization = %q", gotAuth)
	}
}

func TestBindChatStructured(t *testing.T) {
	want := json.RawMessage(`{"memories":[]}`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/structured" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]json.RawMessage{"data": want})
	}))
	defer srv.Close()

	reg := New(provider.NewHub(0), "")
	registerProvider(t, reg, "stub", srv.URL, false, true, false)

	p := BindChat(reg, func() (string, string, error) { return "stub", "m", nil })
	s, ok := p.(provider.Structured)
	if !ok {
		t.Fatal("BindChat should implement Structured")
	}
	got, err := s.ChatStructured(context.Background(), nil, provider.JSONSchema{Name: "memories"})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestBindEmbedSendsModel(t *testing.T) {
	var gotModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embed" {
			t.Errorf("path = %s", r.URL.Path)
		}
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		var body struct {
			Model string `json:"model"`
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Error(err)
		}
		gotModel = body.Model
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"vectors": [][]float32{{0.1}}})
	}))
	defer srv.Close()

	reg := New(provider.NewHub(0), "")
	registerProvider(t, reg, "stub", srv.URL, false, false, true)

	e := BindEmbed(reg, func() (string, string, error) { return "stub", "emb-1", nil })
	vecs, err := e.Embed(context.Background(), []string{"hi"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vecs) != 1 || len(vecs[0]) != 1 || vecs[0][0] != 0.1 {
		t.Errorf("vecs = %v", vecs)
	}
	if gotModel != "emb-1" {
		t.Errorf("model = %q, want emb-1", gotModel)
	}
}

func TestBindChatUnknownProvider(t *testing.T) {
	p := BindChat(New(provider.NewHub(0), ""), func() (string, string, error) {
		return "ollama", "llama3.2", nil
	})
	_, err := p.Chat(context.Background(), provider.ChatRequest{})
	if err == nil || !strings.Contains(err.Error(), `unknown provider "ollama"`) {
		t.Fatalf("err = %v, want unknown provider", err)
	}
}

func TestBindChatRereadsConfig(t *testing.T) {
	a := providerServer(t, "from-a")
	defer a.Close()
	b := providerServer(t, "from-b")
	defer b.Close()

	reg := New(provider.NewHub(0), "")
	registerProvider(t, reg, "a", a.URL, true, false, false)
	registerProvider(t, reg, "b", b.URL, true, false, false)

	name := "a"
	p := BindChat(reg, func() (string, string, error) { return name, "m", nil })
	msg, err := p.Chat(context.Background(), provider.ChatRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if msg.Content != "from-a" {
		t.Fatalf("content = %q, want from-a", msg.Content)
	}
	name = "b"
	msg, err = p.Chat(context.Background(), provider.ChatRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if msg.Content != "from-b" {
		t.Fatalf("content = %q, want from-b", msg.Content)
	}
}

func TestBindChatConfigError(t *testing.T) {
	want := errors.New("conf boom")
	p := BindChat(New(provider.NewHub(0), ""), func() (string, string, error) {
		return "", "", want
	})
	_, err := p.Chat(context.Background(), provider.ChatRequest{})
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want conf boom", err)
	}
}

func TestBindRejectsMissingRoute(t *testing.T) {
	reg := New(provider.NewHub(0), "")
	registerProvider(t, reg, "embed-only", "http://127.0.0.1:9", false, false, true)
	registerProvider(t, reg, "chat-only", "http://127.0.0.1:10", true, false, false)

	_, err := BindChat(reg, func() (string, string, error) {
		return "embed-only", "m", nil
	}).Chat(context.Background(), provider.ChatRequest{})
	if err == nil || err.Error() != `provider "embed-only" does not support chat` {
		t.Fatalf("chat err = %v", err)
	}

	_, err = BindEmbed(reg, func() (string, string, error) {
		return "chat-only", "m", nil
	}).Embed(context.Background(), []string{"hi"})
	if err == nil || err.Error() != `provider "chat-only" does not support embedding` {
		t.Fatalf("embed err = %v", err)
	}
}

func TestBindChatDropUnresolves(t *testing.T) {
	srv := providerServer(t, "ok")
	defer srv.Close()
	reg := New(provider.NewHub(0), "")
	registerProvider(t, reg, "stub", srv.URL, true, false, false)
	p := BindChat(reg, func() (string, string, error) { return "stub", "m", nil })
	if _, err := p.Chat(context.Background(), provider.ChatRequest{}); err != nil {
		t.Fatal(err)
	}
	reg.Drop("golem-stub")
	_, err := p.Chat(context.Background(), provider.ChatRequest{})
	if err == nil || !strings.Contains(err.Error(), `unknown provider "stub"`) {
		t.Fatalf("err = %v, want unknown provider", err)
	}
}

func TestBindChatExpiryUnresolves(t *testing.T) {
	srv := providerServer(t, "ok")
	defer srv.Close()
	reg := New(provider.NewHub(0), "")
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	reg.Now = func() time.Time { return now }
	reg.TTL = time.Minute
	registerProvider(t, reg, "stub", srv.URL, true, false, false)

	p := BindChat(reg, func() (string, string, error) { return "stub", "m", nil })
	if _, err := p.Chat(context.Background(), provider.ChatRequest{}); err != nil {
		t.Fatal(err)
	}
	now = now.Add(30 * time.Second)
	if err := reg.Heartbeat("golem-stub"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	_, err := p.Chat(context.Background(), provider.ChatRequest{})
	if err == nil || !strings.Contains(err.Error(), `unknown provider "stub"`) {
		t.Fatalf("err = %v, want unknown provider", err)
	}
}

func registerProvider(t *testing.T, reg *Registry, id, callback string, chat, structured, embed bool) {
	t.Helper()
	err := reg.Register(Registration{
		Name:        "golem-" + id,
		CallbackURL: callback,
		Capabilities: []Capability{{
			Kind:       "provider",
			ID:         id,
			Chat:       chat,
			Structured: structured,
			Embed:      embed,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func providerServer(t *testing.T, content string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(provider.Message{Role: "assistant", Content: content})
	}))
}
