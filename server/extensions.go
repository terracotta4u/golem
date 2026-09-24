package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/terracotta4u/golem/registry"
)

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

	var req registry.Registration
	if err := json.NewDecoder(io.LimitReader(r.Body, maxBody)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if err := s.reg.Register(req); err != nil {
		writeJSON(w, registryStatus(err), map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleHeartbeatExtension(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, maxBody)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if err := s.reg.Heartbeat(req.Name); err != nil {
		writeJSON(w, registryStatus(err), map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleListExtensions(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"extensions": s.reg.List()})
}

func registryStatus(err error) int {
	switch {
	case errors.Is(err, registry.ErrConflict):
		return http.StatusConflict
	case errors.Is(err, registry.ErrNotFound):
		return http.StatusNotFound
	default:
		return http.StatusBadRequest
	}
}
