// Package registry is the live table of extension processes.
package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/terracotta4u/golem/provider"
	"github.com/terracotta4u/golem/provider/remote"
	"github.com/terracotta4u/golem/tool"
)

const extensionTTL = 30 * time.Second

var (
	ErrInvalid  = errors.New("invalid")
	ErrConflict = errors.New("conflict")
	ErrNotFound = errors.New("not found")
)

// Registration is one extension process and the capabilities it advertises.
type Registration struct {
	Name         string       `json:"name"`
	CallbackURL  string       `json:"callback_url"`
	Capabilities []Capability `json:"capabilities"`
}

// Capability is a provider, a tool, or an unknown kind stored for later.
type Capability struct {
	Kind        string          `json:"kind"`
	ID          string          `json:"id,omitempty"`
	Chat        bool            `json:"chat,omitempty"`
	Structured  bool            `json:"structured,omitempty"`
	Embed       bool            `json:"embed,omitempty"`
	Name        string          `json:"name,omitempty"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// Extension is a live registration.
type Extension struct {
	Name         string       `json:"name"`
	CallbackURL  string       `json:"callback_url"`
	Capabilities []Capability `json:"capabilities"`
}

type extRecord struct {
	name     string
	callback string
	caps     []Capability
	expiry   time.Time
}

type failure struct {
	kind error
	msg  string
}

func (e *failure) Error() string { return e.msg }

func (e *failure) Unwrap() error { return e.kind }

func fail(kind error, format string, args ...any) error {
	return &failure{kind: kind, msg: fmt.Sprintf(format, args...)}
}

// Registry tracks extensions until they expire or are dropped.
// Now and TTL are settable so tests can expire a record.
type Registry struct {
	Now   func() time.Time
	TTL   time.Duration
	hub   *provider.Hub
	token string

	mu   sync.Mutex
	exts map[string]*extRecord
}

func New(hub *provider.Hub, token string) *Registry {
	return &Registry{
		Now:   time.Now,
		TTL:   extensionTTL,
		hub:   hub,
		token: token,
		exts:  make(map[string]*extRecord),
	}
}

// Register stores req and publishes its providers on the hub.
// A registration with the same name replaces the previous one.
func (r *Registry) Register(req Registration) error {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return fail(ErrInvalid, "name is required")
	}
	callback, err := parseCallback(req.CallbackURL)
	if err != nil {
		return fail(ErrInvalid, "%s", err.Error())
	}
	caps, err := normalizeCaps(req.Capabilities)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.dropLocked(name)
	if err := r.toolConflictLocked(caps); err != nil {
		return err
	}

	// Put each provider capability into the hub so later Chat/Embed lookups
	// can POST to this extension. One remote client is shared; the model name
	// is filled in at call time via ForModel.
	client := remote.New(callback, r.token)
	var registered []string
	rollback := func() {
		for _, id := range registered {
			r.hub.Unregister(id)
		}
	}
	for _, cap := range caps {
		if cap.Kind != "provider" {
			continue
		}
		b := provider.Backend{}
		if cap.Chat || cap.Structured {
			b.Chat = provider.NewModelChat(func(model string) provider.Provider {
				return client.ForModel(model)
			})
		}
		if cap.Embed {
			b.Embedder = provider.NewModelEmbedder(func(model string) provider.Embedder {
				return client.ForModel(model)
			})
		}
		if b.Chat == nil && b.Embedder == nil {
			rollback()
			return fail(ErrInvalid, "provider must advertise chat or embed")
		}
		if err := r.hub.Register(cap.ID, b); err != nil {
			rollback()
			kind := ErrInvalid
			if strings.Contains(err.Error(), "already registered") {
				kind = ErrConflict
			}
			return fail(kind, "%s", err.Error())
		}
		registered = append(registered, cap.ID)
	}

	r.exts[name] = &extRecord{
		name:     name,
		callback: callback,
		caps:     caps,
		expiry:   r.deadline(),
	}
	return nil
}

// Heartbeat refreshes the expiry of a live extension.
func (r *Registry) Heartbeat(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fail(ErrInvalid, "name is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.liveLocked(name)
	if !ok {
		return fail(ErrNotFound, "extension not found")
	}
	e.expiry = r.deadline()
	return nil
}

// List returns extensions that are still inside their TTL.
func (r *Registry) List() []Extension {
	r.mu.Lock()
	defer r.mu.Unlock()
	names := make([]string, 0, len(r.exts))
	for name := range r.exts {
		names = append(names, name)
	}
	list := make([]Extension, 0, len(names))
	for _, name := range names {
		e, ok := r.liveLocked(name)
		if !ok {
			continue
		}
		list = append(list, Extension{
			Name:         e.name,
			CallbackURL:  e.callback,
			Capabilities: append([]Capability(nil), e.caps...),
		})
	}
	return list
}

// Drop removes name and unregisters its providers.
func (r *Registry) Drop(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dropLocked(name)
}

func (r *Registry) toolConflictLocked(caps []Capability) error {
	taken := map[string]struct{}{}
	for _, cap := range caps {
		if cap.Kind != "tool" {
			continue
		}
		if tool.Reserved(cap.Name) {
			return fail(ErrConflict, "tool %q conflicts with a builtin", cap.Name)
		}
		if _, ok := taken[cap.Name]; ok {
			return fail(ErrConflict, "tool %q is already registered", cap.Name)
		}
		taken[cap.Name] = struct{}{}
	}
	if len(taken) == 0 {
		return nil
	}
	for name, e := range r.exts {
		if !r.fresh(e) {
			r.dropLocked(name)
			continue
		}
		for _, cap := range e.caps {
			if cap.Kind == "tool" {
				if _, ok := taken[cap.Name]; ok {
					return fail(ErrConflict, "tool %q is already registered", cap.Name)
				}
			}
		}
	}
	return nil
}

func (r *Registry) fresh(e *extRecord) bool {
	return e.expiry.IsZero() || r.now().Before(e.expiry)
}

func (r *Registry) dropLocked(name string) {
	e, ok := r.exts[name]
	if !ok {
		return
	}
	for _, cap := range e.caps {
		if cap.Kind == "provider" && cap.ID != "" {
			r.hub.Unregister(cap.ID)
		}
	}
	delete(r.exts, name)
}

func (r *Registry) liveLocked(name string) (*extRecord, bool) {
	e, ok := r.exts[name]
	if !ok {
		return nil, false
	}
	if !r.fresh(e) {
		r.dropLocked(name)
		return nil, false
	}
	return e, true
}

func (r *Registry) deadline() time.Time {
	if r.TTL <= 0 {
		return time.Time{}
	}
	return r.now().Add(r.TTL)
}

func (r *Registry) now() time.Time {
	if r.Now == nil {
		return time.Now()
	}
	return r.Now()
}

func normalizeCaps(caps []Capability) ([]Capability, error) {
	out := make([]Capability, 0, len(caps))
	for _, cap := range caps {
		cap.Kind = strings.TrimSpace(cap.Kind)
		cap.ID = strings.TrimSpace(cap.ID)
		if cap.Kind == "" {
			return nil, fail(ErrInvalid, "capability kind is required")
		}
		if cap.Kind == "provider" && cap.ID == "" {
			return nil, fail(ErrInvalid, "provider id is required")
		}
		if cap.Kind == "provider" && !cap.Chat && !cap.Structured && !cap.Embed {
			return nil, fail(ErrInvalid, "provider must advertise chat or embed")
		}
		if cap.Kind == "tool" {
			cap.Name = strings.TrimSpace(cap.Name)
			cap.Description = strings.TrimSpace(cap.Description)
			if cap.Name == "" {
				return nil, fail(ErrInvalid, "tool name is required")
			}
			if _, err := objectParams(cap.Parameters); err != nil {
				return nil, fail(ErrInvalid, "%s", err.Error())
			}
		}
		out = append(out, cap)
	}
	return out, nil
}

func parseCallback(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || u.Host == "" || u.User != nil {
		return "", fmt.Errorf("callback_url must be http://127.0.0.1, localhost, or [::1]")
	}
	switch u.Hostname() {
	case "127.0.0.1", "localhost", "::1":
		return strings.TrimRight(raw, "/"), nil
	default:
		return "", fmt.Errorf("callback_url must be http://127.0.0.1, localhost, or [::1]")
	}
}
