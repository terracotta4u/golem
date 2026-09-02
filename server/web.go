package server

import (
	"embed"
	"errors"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/google/uuid"

	"github.com/terracotta4u/golem/store"
)

const webChannel = "web"

//go:embed web/templates/*.html web/templates/components/*.html web/static/*
var webFS embed.FS

func parseWeb() *template.Template {
	return template.Must(template.ParseFS(webFS, "web/templates/*.html", "web/templates/components/*.html"))
}

func (s *Server) mountWeb(mux *http.ServeMux) {
	static, err := fs.Sub(webFS, "web/static")
	if err != nil {
		panic(err)
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(static))))
	mux.HandleFunc("GET /{$}", s.handleHome)
	mux.HandleFunc("POST /conversations", s.handleNewConversation)
	mux.HandleFunc("GET /conversations/{id}", s.handleConversation)
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	var list []store.Conversation
	if s.opts.Store != nil {
		all, err := s.opts.Store.List()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, c := range all {
			if c.Channel == webChannel {
				list = append(list, c)
			}
		}
	}
	s.render(w, "home", map[string]any{"Conversations": list})
}

func (s *Server) handleNewConversation(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/conversations/"+uuid.NewString(), http.StatusSeeOther)
}

func (s *Server) handleConversation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	conv := store.Conversation{ID: id, Channel: webChannel}
	if s.opts.Store != nil {
		c, err := s.opts.Store.Load(id)
		switch {
		case errors.Is(err, store.ErrNotFound):
		case err != nil:
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		case c.Channel != webChannel:
			http.NotFound(w, r)
			return
		default:
			conv = c
		}
	}
	s.render(w, "conversation", conv)
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
