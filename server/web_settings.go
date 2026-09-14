package server

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/terracotta4u/golem/conf"
)

func (s *Server) mountWebSettings(mux *http.ServeMux) {
	mux.HandleFunc("GET /settings", s.handleSettings)
	mux.HandleFunc("GET /settings/general", s.handleSettingsGeneral)
	mux.HandleFunc("POST /settings/general", s.handleSettingsSave)
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	s.render(w, "settings", map[string]any{
		"Title": "Settings",
	})
}

func (s *Server) handleSettingsGeneral(w http.ResponseWriter, r *http.Request) {
	cfg, _, err := conf.Load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, "settings-general", map[string]any{
		"Title":         "General",
		"Provider":      cfg.DefaultModel.Provider,
		"Model":         cfg.DefaultModel.Model,
		"MaxToolRounds": cfg.MaxToolRounds,
	})
}

func (s *Server) handleSettingsSave(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	cfg, _, err := conf.Load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	provider := strings.TrimSpace(r.FormValue("provider"))
	if provider == "" {
		provider = "openrouter"
	}
	if provider != "openrouter" {
		http.Error(w, "unsupported provider", http.StatusBadRequest)
		return
	}

	model := strings.TrimSpace(r.FormValue("model"))
	if model == "" {
		http.Error(w, "model is required", http.StatusBadRequest)
		return
	}

	rawRounds := strings.TrimSpace(r.FormValue("max_tool_rounds"))
	maxRounds := 0
	if rawRounds != "" {
		maxRounds, err = strconv.Atoi(rawRounds)
		if err != nil || maxRounds < 0 {
			http.Error(w, "max_tool_rounds must be a non-negative integer", http.StatusBadRequest)
			return
		}
	}

	cfg.DefaultModel.Provider = provider
	cfg.DefaultModel.Model = model
	cfg.MaxToolRounds = maxRounds
	if err := conf.Save(cfg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/settings/general", http.StatusSeeOther)
}
