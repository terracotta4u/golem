package provider

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type Backend struct {
	Chat     Provider
	Embedder Embedder
}

type Hub struct {
	ttl     time.Duration
	now     func() time.Time
	mu      sync.Mutex
	entries map[string]*hubEntry
}

type hubEntry struct {
	backend Backend
	expiry  time.Time
}

func NewHub(ttl time.Duration) *Hub {
	return &Hub{
		ttl:     ttl,
		now:     time.Now,
		entries: make(map[string]*hubEntry),
	}
}

func (h *Hub) Register(id string, b Backend) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("missing provider id")
	}
	if b.Chat == nil && b.Embedder == nil {
		return fmt.Errorf("provider %q: missing chat and embedding", id)
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.live(id); ok {
		return fmt.Errorf("provider %q already registered", id)
	}
	h.entries[id] = &hubEntry{backend: b, expiry: h.deadline()}
	return nil
}

func (h *Hub) Get(id string) (Backend, error) {
	id = strings.TrimSpace(id)
	h.mu.Lock()
	defer h.mu.Unlock()
	e, ok := h.live(id)
	if !ok {
		return Backend{}, fmt.Errorf("unknown provider %q", id)
	}
	return e.backend, nil
}

func (h *Hub) Unregister(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.entries, strings.TrimSpace(id))
}

func (h *Hub) Heartbeat(id string) error {
	id = strings.TrimSpace(id)
	h.mu.Lock()
	defer h.mu.Unlock()
	e, ok := h.live(id)
	if !ok {
		return fmt.Errorf("unknown provider %q", id)
	}
	e.expiry = h.deadline()
	return nil
}

func (h *Hub) deadline() time.Time {
	if h.ttl <= 0 {
		return time.Time{}
	}
	return h.now().Add(h.ttl)
}

func (h *Hub) live(id string) (*hubEntry, bool) {
	e, ok := h.entries[id]
	if !ok {
		return nil, false
	}
	if !e.expiry.IsZero() && !h.now().Before(e.expiry) {
		delete(h.entries, id)
		return nil, false
	}
	return e, true
}
