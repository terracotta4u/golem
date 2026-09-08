package server

import (
	"net/http"
)

func (s *Server) mountWebSettings(mux *http.ServeMux) {
	mux.HandleFunc("GET /settings", s.handleSettings)
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	list, err := s.webConversations()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, "settings", map[string]any{
		"Title":         "Settings",
		"Conversations": list,
	})
}
