package server

import (
	"context"
	"net/http"

	"github.com/terracotta4u/golem/server/api"
)

func (s *Server) routes(runCtx context.Context) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /static/", s.pages.Static())
	mux.HandleFunc("GET /login", s.loginPage)
	mux.HandleFunc("POST /login", s.loginSubmit)

	// API routes
	mux.Handle("GET /v1/health", s.bearer(http.HandlerFunc(api.Health)))
	mux.Handle("POST /v1/conversations/{id}/turns", s.bearer(s.chat.Post(runCtx)))
	mux.Handle("GET /v1/turns/{id}", s.bearer(http.HandlerFunc(s.chat.Get)))
	mux.Handle("POST /v1/extensions/register", s.bearer(http.HandlerFunc(s.regAPI.Register)))
	mux.Handle("POST /v1/extensions/heartbeat", s.bearer(http.HandlerFunc(s.regAPI.Heartbeat)))
	mux.Handle("GET /v1/extensions", s.bearer(http.HandlerFunc(s.regAPI.List)))

	// Web routes. Static files and the sign-in page stay outside the session.
	mux.Handle("GET /{$}", s.web(http.HandlerFunc(s.webChat.Home)))
	mux.Handle("GET /conversations/{id}", s.web(http.HandlerFunc(s.webChat.Conversation)))
	mux.Handle("POST /conversations/{id}/turns", s.web(s.webChat.Post(runCtx)))
	mux.Handle("GET /turns/{id}", s.web(http.HandlerFunc(s.webChat.Events)))
	mux.Handle("GET /settings", s.web(http.HandlerFunc(s.settings.Page)))
	mux.Handle("GET /settings/general", s.web(http.HandlerFunc(s.settings.General)))
	mux.Handle("POST /settings/general", s.web(http.HandlerFunc(s.settings.Save)))
	mux.Handle("GET /settings/about", s.web(http.HandlerFunc(s.settings.About)))
	mux.Handle("GET /settings/extensions", s.web(http.HandlerFunc(s.extensions.List)))
	mux.Handle("GET /settings/extensions/add", s.web(http.HandlerFunc(s.extensions.Add)))
	mux.Handle("POST /settings/extensions/add/url", s.web(http.HandlerFunc(s.extensions.AddURL)))
	mux.Handle("POST /settings/extensions/add/archive", s.web(http.HandlerFunc(s.extensions.AddArchive)))
	mux.Handle("GET /settings/extensions/{name}", s.web(http.HandlerFunc(s.extensions.Detail)))
	mux.Handle("POST /settings/extensions/{name}/remove", s.web(http.HandlerFunc(s.extensions.Remove)))
	mux.Handle("POST /logout", s.web(http.HandlerFunc(s.logout)))

	return s.localRequests(mux)
}
