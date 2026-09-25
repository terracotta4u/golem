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

// providerCap is one provider a live extension advertised.
// client is shared by every provider on that extension; the model is chosen per call.
type providerCap struct {
	id         string
	chat       bool
	structured bool
	embed      bool
	client     *remote.Client
}

func (p providerCap) supportsChat() bool {
	return p.chat || p.structured
}

// liveTool is one tool a live extension advertised.
type liveTool struct {
	name        string
	description string
	parameters  json.RawMessage
}

type extRecord struct {
	name      string
	callback  string
	caps      []Capability // registration order, returned by List
	providers []providerCap
	tools     []liveTool
	expiry    time.Time
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
	token string

	mu   sync.Mutex
	exts map[string]*extRecord
}

func New(token string) *Registry {
	return &Registry{
		Now:   time.Now,
		TTL:   extensionTTL,
		token: token,
		exts:  make(map[string]*extRecord),
	}
}

// SetToken sets the bearer token sent when calling an extension.
// Call it before any registration. A client is built from the token at register time.
func (r *Registry) SetToken(token string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.token = token
}

// Register stores req. A registration with the same name replaces the previous one.
// One remote client is shared by the extension's providers; the model is chosen per call.
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
	if err := r.providerConflictLocked(caps); err != nil {
		return err
	}

	client := remote.New(callback, r.token)
	var providers []providerCap
	var tools []liveTool
	for _, cap := range caps {
		switch cap.Kind {
		case "tool":
			tools = append(tools, liveTool{
				name:        cap.Name,
				description: cap.Description,
				parameters:  append(json.RawMessage(nil), cap.Parameters...),
			})
		case "provider":
			providers = append(providers, providerCap{
				id:         cap.ID,
				chat:       cap.Chat,
				structured: cap.Structured,
				embed:      cap.Embed,
				client:     client,
			})
		}
	}

	r.exts[name] = &extRecord{
		name:      name,
		callback:  callback,
		caps:      caps,
		providers: providers,
		tools:     tools,
		expiry:    r.deadline(),
	}
	return nil
}

// provider returns a live provider capability with id.
func (r *Registry) provider(id string) (providerCap, error) {
	id = strings.TrimSpace(id)
	r.mu.Lock()
	defer r.mu.Unlock()
	for name, e := range r.exts {
		if !r.fresh(e) {
			r.dropLocked(name)
			continue
		}
		for _, p := range e.providers {
			if p.id == id {
				return p, nil
			}
		}
	}
	return providerCap{}, fmt.Errorf("unknown provider %q", id)
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
		for _, tc := range e.tools {
			if _, ok := taken[tc.name]; ok {
				return fail(ErrConflict, "tool %q is already registered", tc.name)
			}
		}
	}
	return nil
}

func (r *Registry) providerConflictLocked(caps []Capability) error {
	taken := map[string]struct{}{}
	for _, cap := range caps {
		if cap.Kind != "provider" {
			continue
		}
		if _, ok := taken[cap.ID]; ok {
			return fail(ErrConflict, "provider %q already registered", cap.ID)
		}
		taken[cap.ID] = struct{}{}
	}
	if len(taken) == 0 {
		return nil
	}
	for name, e := range r.exts {
		if !r.fresh(e) {
			r.dropLocked(name)
			continue
		}
		for _, p := range e.providers {
			if _, ok := taken[p.id]; ok {
				return fail(ErrConflict, "provider %q already registered", p.id)
			}
		}
	}
	return nil
}

func (r *Registry) fresh(e *extRecord) bool {
	return e.expiry.IsZero() || r.now().Before(e.expiry)
}

func (r *Registry) dropLocked(name string) {
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
