package provider

import (
	"strings"
	"testing"
	"time"
)

func TestHubRegisterAndGet(t *testing.T) {
	h := NewHub(time.Minute)
	chat := stubProvider{reply: Message{Role: "assistant", Content: "ok"}}
	embed := stubEmbedder{vecs: [][]float32{{0.1}}}
	if err := h.Register("stub", Backend{Chat: chat, Embedder: embed}); err != nil {
		t.Fatal(err)
	}

	got, err := h.Get("stub")
	if err != nil {
		t.Fatal(err)
	}
	if got.Chat == nil || got.Embedder == nil {
		t.Fatalf("backend = %+v, want chat and embedder", got)
	}
}

func TestHubUnknownProvider(t *testing.T) {
	h := NewHub(time.Minute)
	if err := h.Register("stub", Backend{Chat: stubProvider{}}); err != nil {
		t.Fatal(err)
	}

	_, err := h.Get("openrouter")
	if err == nil {
		t.Fatal("want error for unknown provider")
	}
	if !strings.Contains(err.Error(), "openrouter") {
		t.Errorf("err = %v, want unknown provider named", err)
	}
}

func TestHubDuplicateRegister(t *testing.T) {
	h := NewHub(time.Minute)
	if err := h.Register("stub", Backend{Chat: stubProvider{}}); err != nil {
		t.Fatal(err)
	}

	err := h.Register("stub", Backend{Chat: stubProvider{}})
	if err == nil {
		t.Fatal("want error for duplicate register")
	}
	if !strings.Contains(err.Error(), "stub") {
		t.Errorf("err = %v, want duplicate id", err)
	}
}

func TestHubUnregister(t *testing.T) {
	h := NewHub(time.Minute)
	if err := h.Register("stub", Backend{Chat: stubProvider{}}); err != nil {
		t.Fatal(err)
	}
	h.Unregister("stub")

	if _, err := h.Get("stub"); err == nil {
		t.Fatal("want error after unregister")
	}
	if err := h.Register("stub", Backend{Chat: stubProvider{}}); err != nil {
		t.Fatal(err)
	}
}

func TestHubHeartbeatExpires(t *testing.T) {
	h := NewHub(time.Minute)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	h.now = func() time.Time { return now }
	if err := h.Register("stub", Backend{Chat: stubProvider{}}); err != nil {
		t.Fatal(err)
	}

	now = now.Add(30 * time.Second)
	if err := h.Heartbeat("stub"); err != nil {
		t.Fatal(err)
	}

	now = now.Add(time.Minute)
	if _, err := h.Get("stub"); err == nil {
		t.Fatal("want error after ttl")
	}
}

func TestHubHeartbeatUnknown(t *testing.T) {
	h := NewHub(time.Minute)
	err := h.Heartbeat("stub")
	if err == nil {
		t.Fatal("want error for unknown provider")
	}
	if !strings.Contains(err.Error(), "stub") {
		t.Errorf("err = %v, want unknown provider named", err)
	}
}

func TestHubRegisterAfterExpiry(t *testing.T) {
	h := NewHub(time.Minute)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	h.now = func() time.Time { return now }
	if err := h.Register("stub", Backend{Chat: stubProvider{}}); err != nil {
		t.Fatal(err)
	}

	now = now.Add(time.Minute)
	if err := h.Register("stub", Backend{Chat: stubProvider{}}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Get("stub"); err != nil {
		t.Fatal(err)
	}
}

func TestHubZeroTTLNeverExpires(t *testing.T) {
	h := NewHub(0)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	h.now = func() time.Time { return now }
	if err := h.Register("openrouter", Backend{Chat: stubProvider{}}); err != nil {
		t.Fatal(err)
	}

	now = now.Add(24 * time.Hour)
	if _, err := h.Get("openrouter"); err != nil {
		t.Fatal(err)
	}
}

func TestHubRegisterRequiresIDAndBackend(t *testing.T) {
	h := NewHub(time.Minute)
	if err := h.Register("  ", Backend{Chat: stubProvider{}}); err == nil {
		t.Fatal("want error for empty id")
	}
	if err := h.Register("stub", Backend{}); err == nil {
		t.Fatal("want error for empty backend")
	}
}
