package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/terracotta4u/golem/provider"
	"github.com/terracotta4u/golem/provider/remote"
	"github.com/terracotta4u/golem/tool"
)

const extensionTTL = 30 * time.Second

type capability struct {
	Kind        string          `json:"kind"`
	ID          string          `json:"id,omitempty"`
	Chat        bool            `json:"chat,omitempty"`
	Structured  bool            `json:"structured,omitempty"`
	Embed       bool            `json:"embed,omitempty"`
	Name        string          `json:"name,omitempty"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type extRecord struct {
	name     string
	callback string
	caps     []capability
	expiry   time.Time
}

type registerRequest struct {
	Name         string       `json:"name"`
	CallbackURL  string       `json:"callback_url"`
	Capabilities []capability `json:"capabilities"`
}

type heartbeatRequest struct {
	Name string `json:"name"`
}

type extJSON struct {
	Name         string       `json:"name"`
	CallbackURL  string       `json:"callback_url"`
	Capabilities []capability `json:"capabilities"`
}

func (s *Server) mountExtensions(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/extensions/register", s.handleRegisterExtension)
	mux.HandleFunc("POST /v1/extensions/heartbeat", s.handleHeartbeatExtension)
	mux.HandleFunc("GET /v1/extensions", s.handleListExtensions)
}

func (s *Server) handleRegisterExtension(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req registerRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, maxBody)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	callback, err := parseCallback(req.CallbackURL)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	caps, err := normalizeCaps(req.Capabilities)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.dropLocked(name)
	if err := s.toolConflictLocked(caps); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}

	// Put each provider capability into the hub so later Chat/Embed lookups
	// can POST to this extension. One remote client is shared; the model name
	// is filled in at call time via ForModel.
	client := remote.New(callback, s.opts.Token)
	var registered []string
	rollback := func() {
		for _, id := range registered {
			s.hub.Unregister(id)
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
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "provider must advertise chat or embed"})
			return
		}
		if err := s.hub.Register(cap.ID, b); err != nil {
			// Undo ids from this request only; dropLocked already cleared the old name.
			rollback()
			status := http.StatusBadRequest
			if strings.Contains(err.Error(), "already registered") {
				status = http.StatusConflict
			}
			writeJSON(w, status, map[string]string{"error": err.Error()})
			return
		}
		registered = append(registered, cap.ID)
	}

	s.exts[name] = &extRecord{
		name:     name,
		callback: callback,
		caps:     caps,
		expiry:   s.deadline(),
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleHeartbeatExtension(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req heartbeatRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, maxBody)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.liveLocked(name)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "extension not found"})
		return
	}
	e.expiry = s.deadline()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleListExtensions(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	names := make([]string, 0, len(s.exts))
	for name := range s.exts {
		names = append(names, name)
	}
	list := make([]extJSON, 0, len(names))
	for _, name := range names {
		e, ok := s.liveLocked(name)
		if !ok {
			continue
		}
		list = append(list, extJSON{
			Name:         e.name,
			CallbackURL:  e.callback,
			Capabilities: e.caps,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"extensions": list})
}

func (s *Server) toolConflictLocked(caps []capability) error {
	taken := map[string]struct{}{}
	for _, cap := range caps {
		if cap.Kind != "tool" {
			continue
		}
		if tool.Reserved(cap.Name) {
			return fmt.Errorf("tool %q conflicts with a builtin", cap.Name)
		}
		if _, ok := taken[cap.Name]; ok {
			return fmt.Errorf("tool %q is already registered", cap.Name)
		}
		taken[cap.Name] = struct{}{}
	}
	if len(taken) == 0 {
		return nil
	}
	for name, e := range s.exts {
		if !s.fresh(e) {
			s.dropLocked(name)
			continue
		}
		for _, cap := range e.caps {
			if cap.Kind == "tool" {
				if _, ok := taken[cap.Name]; ok {
					return fmt.Errorf("tool %q is already registered", cap.Name)
				}
			}
		}
	}
	return nil
}

func (s *Server) fresh(e *extRecord) bool {
	return e.expiry.IsZero() || s.now().Before(e.expiry)
}

func (s *Server) dropLocked(name string) {
	e, ok := s.exts[name]
	if !ok {
		return
	}
	for _, cap := range e.caps {
		if cap.Kind == "provider" && cap.ID != "" {
			s.hub.Unregister(cap.ID)
		}
	}
	delete(s.exts, name)
}

func (s *Server) liveLocked(name string) (*extRecord, bool) {
	e, ok := s.exts[name]
	if !ok {
		return nil, false
	}
	if !s.fresh(e) {
		s.dropLocked(name)
		return nil, false
	}
	return e, true
}

func (s *Server) deadline() time.Time {
	if s.ttl <= 0 {
		return time.Time{}
	}
	return s.now().Add(s.ttl)
}

func normalizeCaps(caps []capability) ([]capability, error) {
	out := make([]capability, 0, len(caps))
	for _, cap := range caps {
		cap.Kind = strings.TrimSpace(cap.Kind)
		cap.ID = strings.TrimSpace(cap.ID)
		if cap.Kind == "" {
			return nil, fmt.Errorf("capability kind is required")
		}
		if cap.Kind == "provider" && cap.ID == "" {
			return nil, fmt.Errorf("provider id is required")
		}
		if cap.Kind == "provider" && !cap.Chat && !cap.Structured && !cap.Embed {
			return nil, fmt.Errorf("provider must advertise chat or embed")
		}
		if cap.Kind == "tool" {
			cap.Name = strings.TrimSpace(cap.Name)
			cap.Description = strings.TrimSpace(cap.Description)
			if cap.Name == "" {
				return nil, fmt.Errorf("tool name is required")
			}
			if err := objectParams(cap.Parameters); err != nil {
				return nil, err
			}
		}
		out = append(out, cap)
	}
	return out, nil
}

func objectParams(raw json.RawMessage) error {
	if len(raw) == 0 {
		return fmt.Errorf("tool parameters are required")
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
		return fmt.Errorf("tool parameters must be an object")
	}
	return nil
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
