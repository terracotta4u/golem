package registry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/terracotta4u/golem/provider"
)

func TestRegisterRejectsBuiltin(t *testing.T) {
	r := New("")
	err := r.Register(Registration{
		Name:         "golem-weather",
		CallbackURL:  "http://127.0.0.1:9",
		Capabilities: []Capability{toolCap("read")},
	})
	if !errors.Is(err, ErrConflict) || err.Error() != `tool "read" conflicts with a builtin` {
		t.Fatalf("err = %v", err)
	}
	if len(r.List()) != 0 {
		t.Fatal("rejected tool should not be listed")
	}
}

func TestRegisterRejectsDuplicate(t *testing.T) {
	r := New("")
	if err := r.Register(Registration{
		Name:         "one",
		CallbackURL:  "http://127.0.0.1:9",
		Capabilities: []Capability{toolCap("weather")},
	}); err != nil {
		t.Fatal(err)
	}
	err := r.Register(Registration{
		Name:         "two",
		CallbackURL:  "http://127.0.0.1:10",
		Capabilities: []Capability{toolCap("weather")},
	})
	if !errors.Is(err, ErrConflict) || err.Error() != `tool "weather" is already registered` {
		t.Fatalf("err = %v", err)
	}
	list := r.List()
	if len(list) != 1 || list[0].Name != "one" {
		t.Fatalf("list = %+v", list)
	}
}

func TestRegisterReplacesSameExtension(t *testing.T) {
	r := New("")
	body := Registration{
		Name:         "golem-weather",
		CallbackURL:  "http://127.0.0.1:9",
		Capabilities: []Capability{toolCap("weather")},
	}
	if err := r.Register(body); err != nil {
		t.Fatal(err)
	}
	body.CallbackURL = "http://127.0.0.1:10"
	if err := r.Register(body); err != nil {
		t.Fatal(err)
	}
	list := r.List()
	if len(list) != 1 || list[0].CallbackURL != "http://127.0.0.1:10" || list[0].Capabilities[0].Name != "weather" {
		t.Fatalf("list = %+v", list)
	}
}

func TestRegisterExpiresThenReuse(t *testing.T) {
	r := New("")
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	r.Now = func() time.Time { return now }
	r.TTL = time.Minute

	if err := r.Register(Registration{
		Name:         "one",
		CallbackURL:  "http://127.0.0.1:9",
		Capabilities: []Capability{toolCap("weather")},
	}); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	if len(r.List()) != 0 {
		t.Fatal("want empty list after ttl")
	}
	if err := r.Register(Registration{
		Name:         "two",
		CallbackURL:  "http://127.0.0.1:10",
		Capabilities: []Capability{toolCap("weather")},
	}); err != nil {
		t.Fatal(err)
	}
	list := r.List()
	if len(list) != 1 || list[0].Name != "two" {
		t.Fatalf("list = %+v", list)
	}
}

func TestRegisterRejectsDuplicateProvider(t *testing.T) {
	r := New("")
	body := Registration{
		Name:        "one",
		CallbackURL: "http://127.0.0.1:9",
		Capabilities: []Capability{{
			Kind: "provider",
			ID:   "openrouter",
			Chat: true,
		}},
	}
	if err := r.Register(body); err != nil {
		t.Fatal(err)
	}
	body.Name = "two"
	body.CallbackURL = "http://127.0.0.1:10"
	err := r.Register(body)
	if !errors.Is(err, ErrConflict) || err.Error() != `provider "openrouter" already registered` {
		t.Fatalf("err = %v", err)
	}
	list := r.List()
	if len(list) != 1 || list[0].Name != "one" {
		t.Fatalf("list = %+v", list)
	}
}

func TestDropUnregistersProvider(t *testing.T) {
	r := New("secret")
	if err := r.Register(Registration{
		Name:        "golem-openrouter",
		CallbackURL: "http://127.0.0.1:9",
		Capabilities: []Capability{{
			Kind: "provider",
			ID:   "openrouter",
			Chat: true,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := resolveChat(r, "openrouter"); err == nil || err.Error() == `unknown provider "openrouter"` {
		t.Fatalf("err = %v, want the provider to resolve", err)
	}
	r.Drop("golem-openrouter")
	if err := resolveChat(r, "openrouter"); err == nil || err.Error() != `unknown provider "openrouter"` {
		t.Fatalf("err = %v, want unknown provider", err)
	}
	if len(r.List()) != 0 {
		t.Fatal("dropped extension should not be listed")
	}
}

func TestExpiryUnregistersProvider(t *testing.T) {
	r := New("")
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	r.Now = func() time.Time { return now }
	r.TTL = time.Minute
	if err := r.Register(Registration{
		Name:        "golem-openrouter",
		CallbackURL: "http://127.0.0.1:9",
		Capabilities: []Capability{{
			Kind: "provider",
			ID:   "openrouter",
			Chat: true,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	now = now.Add(30 * time.Second)
	if err := r.Heartbeat("golem-openrouter"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	if len(r.List()) != 0 {
		t.Fatal("want empty list after ttl")
	}
	if err := resolveChat(r, "openrouter"); err == nil || err.Error() != `unknown provider "openrouter"` {
		t.Fatalf("err = %v, want unknown provider", err)
	}
}

func resolveChat(r *Registry, id string) error {
	_, err := BindChat(r, func() (string, string, error) { return id, "m", nil }).Chat(context.Background(), provider.ChatRequest{})
	return err
}

func TestRegisterRejectsInvalid(t *testing.T) {
	r := New("")
	err := r.Register(Registration{CallbackURL: "http://127.0.0.1:9"})
	if !errors.Is(err, ErrInvalid) || err.Error() != "name is required" {
		t.Fatalf("err = %v", err)
	}
	err = r.Register(Registration{
		Name:         "ext",
		CallbackURL:  "http://example.com",
		Capabilities: []Capability{{Kind: "provider", ID: "p", Chat: true}},
	})
	if !errors.Is(err, ErrInvalid) || err.Error() != "callback_url must be http://127.0.0.1, localhost, or [::1]" {
		t.Fatalf("err = %v", err)
	}
	err = r.Heartbeat("missing")
	if !errors.Is(err, ErrNotFound) || err.Error() != "extension not found" {
		t.Fatalf("err = %v", err)
	}
}
