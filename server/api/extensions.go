package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/terracotta4u/golem/registry"
)

// Extensions serves the JSON registration routes.
type Extensions struct {
	reg *registry.Registry
}

func NewExtensions(reg *registry.Registry) *Extensions {
	return &Extensions{reg: reg}
}

func (h *Extensions) Register(w http.ResponseWriter, r *http.Request) {
	var req registry.Registration
	if err := json.NewDecoder(io.LimitReader(r.Body, maxBody)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if err := h.reg.Register(req); err != nil {
		writeJSON(w, registryStatus(err), map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Extensions) Heartbeat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, maxBody)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if err := h.reg.Heartbeat(req.Name); err != nil {
		writeJSON(w, registryStatus(err), map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Extensions) List(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"extensions": h.reg.List()})
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
