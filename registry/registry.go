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

// Registration is one extension process and what it advertises.
type Registration struct {
	Name        string     `json:"name"`
	CallbackURL string     `json:"callback_url"`
	Providers   []Provider `json:"providers,omitempty"`
	Tools       []Tool     `json:"tools,omitempty"`
	Channels    []Channel  `json:"channels,omitempty"`
}

// Provider is a model backend an extension advertises.
type Provider struct {
	ID         string `json:"id"`
	Chat       bool   `json:"chat,omitempty"`
	Structured bool   `json:"structured,omitempty"`
	Embed      bool   `json:"embed,omitempty"`
}

// Tool is a function an extension advertises.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"`
}

// Channel is a loop the extension runs itself. Golem lists it and does not call it.
type Channel struct {
	ID string `json:"id"`
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

type extRecord struct {
	name      string
	callback  string
	providers []providerCap
	tools     []Tool
	channels  []Channel
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
	providers, err := normalizeProviders(req.Providers)
	if err != nil {
		return err
	}
	tools, err := normalizeTools(req.Tools)
	if err != nil {
		return err
	}
	channels, err := normalizeChannels(req.Channels)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.dropLocked(name)
	if err := r.toolConflictLocked(tools); err != nil {
		return err
	}
	if err := r.providerConflictLocked(providers); err != nil {
		return err
	}

	client := remote.New(callback, r.token)
	var live []providerCap
	for _, p := range providers {
		live = append(live, providerCap{
			id:         p.ID,
			chat:       p.Chat,
			structured: p.Structured,
			embed:      p.Embed,
			client:     client,
		})
	}

	r.exts[name] = &extRecord{
		name:      name,
		callback:  callback,
		providers: live,
		tools:     copyTools(tools),
		channels:  copyChannels(channels),
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

// List returns registrations that are still inside their TTL.
func (r *Registry) List() []Registration {
	r.mu.Lock()
	defer r.mu.Unlock()
	names := make([]string, 0, len(r.exts))
	for name := range r.exts {
		names = append(names, name)
	}
	list := make([]Registration, 0, len(names))
	for _, name := range names {
		e, ok := r.liveLocked(name)
		if !ok {
			continue
		}
		list = append(list, Registration{
			Name:        e.name,
			CallbackURL: e.callback,
			Providers:   advertisedProviders(e.providers),
			Tools:       copyTools(e.tools),
			Channels:    copyChannels(e.channels),
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

func (r *Registry) toolConflictLocked(tools []Tool) error {
	taken := map[string]struct{}{}
	for _, tc := range tools {
		if tool.Reserved(tc.Name) {
			return fail(ErrConflict, "tool %q conflicts with a builtin", tc.Name)
		}
		if _, ok := taken[tc.Name]; ok {
			return fail(ErrConflict, "tool %q is already registered", tc.Name)
		}
		taken[tc.Name] = struct{}{}
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
			if _, ok := taken[tc.Name]; ok {
				return fail(ErrConflict, "tool %q is already registered", tc.Name)
			}
		}
	}
	return nil
}

func (r *Registry) providerConflictLocked(providers []Provider) error {
	taken := map[string]struct{}{}
	for _, p := range providers {
		if _, ok := taken[p.ID]; ok {
			return fail(ErrConflict, "provider %q already registered", p.ID)
		}
		taken[p.ID] = struct{}{}
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

func normalizeProviders(providers []Provider) ([]Provider, error) {
	if len(providers) == 0 {
		return nil, nil
	}
	out := make([]Provider, 0, len(providers))
	for _, p := range providers {
		p.ID = strings.TrimSpace(p.ID)
		if p.ID == "" {
			return nil, fail(ErrInvalid, "provider id is required")
		}
		if !p.Chat && !p.Structured && !p.Embed {
			return nil, fail(ErrInvalid, "provider must advertise chat or embed")
		}
		out = append(out, p)
	}
	return out, nil
}

func normalizeTools(tools []Tool) ([]Tool, error) {
	if len(tools) == 0 {
		return nil, nil
	}
	out := make([]Tool, 0, len(tools))
	for _, tc := range tools {
		tc.Name = strings.TrimSpace(tc.Name)
		tc.Description = strings.TrimSpace(tc.Description)
		if tc.Name == "" {
			return nil, fail(ErrInvalid, "tool name is required")
		}
		if _, err := objectParams(tc.Parameters); err != nil {
			return nil, fail(ErrInvalid, "%s", err.Error())
		}
		out = append(out, tc)
	}
	return out, nil
}

func normalizeChannels(channels []Channel) ([]Channel, error) {
	if len(channels) == 0 {
		return nil, nil
	}
	out := make([]Channel, 0, len(channels))
	seen := map[string]struct{}{}
	for _, ch := range channels {
		ch.ID = strings.TrimSpace(ch.ID)
		if ch.ID == "" {
			return nil, fail(ErrInvalid, "channel id is required")
		}
		if _, ok := seen[ch.ID]; ok {
			return nil, fail(ErrConflict, "channel %q already registered", ch.ID)
		}
		seen[ch.ID] = struct{}{}
		out = append(out, ch)
	}
	return out, nil
}

func advertisedProviders(in []providerCap) []Provider {
	if len(in) == 0 {
		return nil
	}
	out := make([]Provider, len(in))
	for i, p := range in {
		out[i] = Provider{ID: p.id, Chat: p.chat, Structured: p.structured, Embed: p.embed}
	}
	return out
}

func copyTools(in []Tool) []Tool {
	if len(in) == 0 {
		return nil
	}
	out := make([]Tool, len(in))
	for i, tc := range in {
		tc.Parameters = append(json.RawMessage(nil), tc.Parameters...)
		out[i] = tc
	}
	return out
}

func copyChannels(in []Channel) []Channel {
	if len(in) == 0 {
		return nil
	}
	return append([]Channel(nil), in...)
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
