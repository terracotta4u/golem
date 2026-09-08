package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/terracotta4u/golem/conf"
	"github.com/terracotta4u/golem/extension"
)

func (s *Server) mountWebExtensions(mux *http.ServeMux) {
	mux.HandleFunc("GET /settings/extensions", s.handleExtensions)
	mux.HandleFunc("GET /settings/extensions/{name}", s.handleExtension)
	mux.HandleFunc("POST /settings/extensions/{name}/remove", s.handleExtensionRemove)
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

func (s *Server) handleExtension(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || name != filepath.Base(name) {
		http.NotFound(w, r)
		return
	}
	root, err := conf.ExtensionsDir()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	p, err := extension.Load(filepath.Join(root, name))
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
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
	origin := cfg.Extensions[name]
	s.render(w, "extension", map[string]any{
		"Title":         p.Name,
		"Conversations": convs,
		"Name":          p.Name,
		"Version":       p.Version,
		"Source":        origin.Source,
		"Ref":           origin.Ref,
		"Revision":      origin.Revision,
	})
}

func (s *Server) handleExtensionRemove(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	root, err := conf.ExtensionsDir()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := extension.Remove(root, name); err != nil {
		if strings.Contains(err.Error(), "not installed") || strings.Contains(err.Error(), "invalid extension name") {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cfg, _, err := conf.Load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	conf.RemoveExtension(&cfg, name)
	if err := conf.Save(cfg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/settings/extensions", http.StatusSeeOther)
}
