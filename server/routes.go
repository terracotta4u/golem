package server

import (
	"context"
	"net/http"

	"github.com/terracotta4u/golem/server/api"
)

func (s *Server) routes(runCtx context.Context) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /static/", s.pages.Static())

	// API routes
	mux.Handle("GET /v1/health", s.bearer(http.HandlerFunc(api.Health)))
	mux.Handle("GET /v1/conversations", s.bearer(http.HandlerFunc(s.convAPI.List)))
	mux.Handle("POST /v1/conversations/{id}/turns", s.bearer(s.chat.Post(runCtx)))
	mux.Handle("GET /v1/turns/{id}", s.bearer(http.HandlerFunc(s.chat.Get)))
	mux.Handle("POST /v1/extensions/register", s.bearer(http.HandlerFunc(s.regAPI.Register)))
	mux.Handle("POST /v1/extensions/heartbeat", s.bearer(http.HandlerFunc(s.regAPI.Heartbeat)))
	mux.Handle("GET /v1/extensions", s.bearer(http.HandlerFunc(s.regAPI.List)))

	// Web routes
	mux.HandleFunc("GET /{$}", s.webChat.Home)
	mux.HandleFunc("GET /conversations/{id}", s.webChat.Conversation)
	mux.HandleFunc("POST /conversations/{id}/turns", s.webChat.Post(runCtx))
	mux.HandleFunc("GET /turns/{id}", s.webChat.Events)
	mux.HandleFunc("GET /settings", s.settings.Page)
	mux.HandleFunc("GET /settings/general", s.settings.General)
	mux.HandleFunc("POST /settings/general", s.settings.Save)
	mux.HandleFunc("GET /settings/about", s.settings.About)
	mux.HandleFunc("GET /settings/extensions", s.extensions.List)
	mux.HandleFunc("GET /settings/extensions/add", s.extensions.Add)
	mux.HandleFunc("POST /settings/extensions/add/url", s.extensions.AddURL)
	mux.HandleFunc("POST /settings/extensions/add/archive", s.extensions.AddArchive)
	mux.HandleFunc("GET /settings/extensions/{name}", s.extensions.Detail)
	mux.HandleFunc("POST /settings/extensions/{name}/remove", s.extensions.Remove)

	return mux
}
