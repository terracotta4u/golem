package web

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/terracotta4u/golem/conf"
	"github.com/terracotta4u/golem/release"
)

const maxBody = 1 << 20

// Settings serves the settings pages.
// Update reports whether a newer release was found. Nil skips the check.
type Settings struct {
	pages   *Pages
	version string
	update  func(context.Context) (release.Status, bool)
}

func NewSettings(pages *Pages, version string, update func(context.Context) (release.Status, bool)) *Settings {
	return &Settings{pages: pages, version: version, update: update}
}

func (h *Settings) Page(w http.ResponseWriter, r *http.Request) {
	h.pages.Render(w, "settings", map[string]any{
		"Title":   "Settings",
		"PageCSS": "settings.css",
	})
}

func (h *Settings) General(w http.ResponseWriter, r *http.Request) {
	cfg, _, err := conf.Load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.pages.Render(w, "settings-general", map[string]any{
		"Title":           "General",
		"DefaultProvider": cfg.DefaultModel.Provider,
		"DefaultModel":    cfg.DefaultModel.Model,
		"FastProvider":    cfg.FastModel.Provider,
		"FastModel":       cfg.FastModel.Model,
		"MaxToolRounds":   cfg.MaxToolRounds,
		"PageCSS":         "settings.css",
	})
}

func (h *Settings) Save(w http.ResponseWriter, r *http.Request) {
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

func (h *Settings) About(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var st release.Status
	var checked bool
	if h.update != nil {
		st, checked = h.update(ctx)
	}
	ver := release.Display(h.version)
	h.pages.Render(w, "settings-about", map[string]any{
		"Title":     "About",
		"Version":   ver,
		"Latest":    release.Display(st.Latest),
		"Available": st.Available,
		"Checked":   checked,
		"Dev":       ver == "dev",
		"Install":   release.InstallCommand,
		"PageCSS":   "settings.css",
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
