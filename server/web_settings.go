package server

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/terracotta4u/golem/conf"
	"github.com/terracotta4u/golem/release"
)

func (s *Server) mountWebSettings(mux *http.ServeMux) {
	mux.HandleFunc("GET /settings", s.handleSettings)
	mux.HandleFunc("GET /settings/general", s.handleSettingsGeneral)
	mux.HandleFunc("POST /settings/general", s.handleSettingsSave)
	mux.HandleFunc("GET /settings/about", s.handleSettingsAbout)
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
		"Title":           "General",
		"DefaultProvider": cfg.DefaultModel.Provider,
		"DefaultModel":    cfg.DefaultModel.Model,
		"FastProvider":    cfg.FastModel.Provider,
		"FastModel":       cfg.FastModel.Model,
		"MaxToolRounds":   cfg.MaxToolRounds,
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

	defaultModel, errMsg := parseModelConfig(r, "default_provider", "default_model")
	if errMsg != "" {
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}
	fastModel, errMsg := parseModelConfig(r, "fast_provider", "fast_model")
	if errMsg != "" {
		http.Error(w, errMsg, http.StatusBadRequest)
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

	cfg.DefaultModel = defaultModel
	cfg.FastModel = fastModel
	cfg.MaxToolRounds = maxRounds
	if err := conf.Save(cfg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/settings/general", http.StatusSeeOther)
}

func (s *Server) handleSettingsAbout(w http.ResponseWriter, r *http.Request) {
	ver := release.Display(s.opts.Version)
	s.render(w, "settings-about", map[string]any{
		"Title":   "About",
		"Version": ver,
		"Dev":     ver == "dev",
	})
}

func parseModelConfig(r *http.Request, providerField, modelField string) (conf.ModelConfig, string) {
	provider := strings.TrimSpace(r.FormValue(providerField))
	if provider == "" {
		return conf.ModelConfig{}, "provider is required"
	}
	model := strings.TrimSpace(r.FormValue(modelField))
	if model == "" {
		return conf.ModelConfig{}, "model is required"
	}
	return conf.ModelConfig{Provider: provider, Model: model}, ""
}
