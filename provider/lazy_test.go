package provider

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestLazyChatUsesModel(t *testing.T) {
	hub := NewHub(0)
	var gotModel string
	if err := hub.Register("stub", Backend{Chat: NewModelChat(func(model string) Provider {
		gotModel = model
		return stubProvider{reply: Message{Role: "assistant", Content: "ok"}}
	})}); err != nil {
		t.Fatal(err)
	}

	p := NewLazyChat(hub, func() (string, string, error) { return "stub", "m1", nil })
	msg, err := p.Chat(context.Background(), ChatRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if msg.Content != "ok" {
		t.Errorf("content = %q, want ok", msg.Content)
	}
	if gotModel != "m1" {
		t.Errorf("model = %q, want m1", gotModel)
	}
}

func TestLazyChatUnknownProvider(t *testing.T) {
	p := NewLazyChat(NewHub(0), func() (string, string, error) { return "ollama", "llama3.2", nil })
	_, err := p.Chat(context.Background(), ChatRequest{})
	if err == nil || !strings.Contains(err.Error(), "ollama") {
		t.Fatalf("err = %v, want unknown ollama", err)
	}
}

func TestLazyChatStructured(t *testing.T) {
	hub := NewHub(0)
	want := json.RawMessage(`{"memories":[]}`)
	if err := hub.Register("stub", Backend{Chat: NewModelChat(func(string) Provider {
		return structuredStub{raw: want}
	})}); err != nil {
		t.Fatal(err)
	}

	p := NewLazyChat(hub, func() (string, string, error) { return "stub", "m", nil })
	s, ok := p.(Structured)
	if !ok {
		t.Fatal("LazyChat should implement Structured")
	}
	got, err := s.ChatStructured(context.Background(), nil, JSONSchema{Name: "memories"})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestLazyEmbedderUsesModel(t *testing.T) {
	hub := NewHub(0)
	var gotModel string
	if err := hub.Register("stub", Backend{Embedder: NewModelEmbedder(func(model string) Embedder {
		gotModel = model
		return stubEmbedder{vecs: [][]float32{{0.1}}}
	})}); err != nil {
		t.Fatal(err)
	}

	e := NewLazyEmbedder(hub, func() (string, string, error) { return "stub", "emb-1", nil })
	vecs, err := e.Embed(context.Background(), []string{"hi"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vecs) != 1 || vecs[0][0] != 0.1 {
		t.Errorf("vecs = %v", vecs)
	}
	if gotModel != "emb-1" {
		t.Errorf("model = %q, want emb-1", gotModel)
	}
}

func TestLazyChatRereadsConfig(t *testing.T) {
	hub := NewHub(time.Minute)
	if err := hub.Register("a", Backend{Chat: stubProvider{reply: Message{Content: "from-a"}}}); err != nil {
		t.Fatal(err)
	}
	if err := hub.Register("b", Backend{Chat: stubProvider{reply: Message{Content: "from-b"}}}); err != nil {
		t.Fatal(err)
	}

	name := "a"
	p := NewLazyChat(hub, func() (string, string, error) { return name, "m", nil })
	msg, err := p.Chat(context.Background(), ChatRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if msg.Content != "from-a" {
		t.Fatalf("content = %q, want from-a", msg.Content)
	}
	name = "b"
	msg, err = p.Chat(context.Background(), ChatRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if msg.Content != "from-b" {
		t.Fatalf("content = %q, want from-b", msg.Content)
	}
}

func TestLazyChatConfigError(t *testing.T) {
	p := NewLazyChat(NewHub(0), func() (string, string, error) { return "", "", errString("conf boom") })
	_, err := p.Chat(context.Background(), ChatRequest{})
	if err == nil || !errors.Is(err, errString("conf boom")) && !strings.Contains(err.Error(), "conf boom") {
		t.Fatalf("err = %v, want conf boom", err)
	}
}

type structuredStub struct {
	raw json.RawMessage
}

func (structuredStub) Chat(context.Context, ChatRequest) (Message, error) {
	return Message{}, nil
}

func (s structuredStub) ChatStructured(context.Context, []Message, JSONSchema) (json.RawMessage, error) {
	return s.raw, nil
}
