package server

import (
	"net/http"

	"github.com/terracotta4u/golem/conf"
	"github.com/terracotta4u/golem/extension"
)

func (s *Server) mountWebExtensions(mux *http.ServeMux) {
	mux.HandleFunc("GET /settings/extensions", s.handleExtensions)
}

func (s *Server) handleExtensions(w http.ResponseWriter, r *http.Request) {
	root, err := conf.ExtensionsDir()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	installed, err := extension.List(root)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cfg, _, err := conf.Load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	convs, err := s.webConversations()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type row struct {
		Name    string
		Version string
		Source  string
	}
	var list []row
	for _, p := range installed {
		list = append(list, row{
			Name:    p.Name,
			Version: p.Version,
			Source:  cfg.Extensions[p.Name].Source,
		})
	}
	s.render(w, "extensions", map[string]any{
		"Title":         "Extensions",
		"Conversations": convs,
		"Extensions":    list,
	})
}
