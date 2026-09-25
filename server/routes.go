package server

import (
	"context"
	"net/http"
)

func (s *Server) routes(runCtx context.Context) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /static/", s.pages.Static())

	// API routes
	mux.Handle("GET /v1/health", s.bearer(http.HandlerFunc(s.handleHealth)))
	mux.Handle("POST /v1/conversations/{id}/turns", s.bearer(s.handlePostTurn(runCtx)))
	mux.Handle("GET /v1/turns/{id}", s.bearer(http.HandlerFunc(s.handleGetTurn)))
	mux.Handle("POST /v1/extensions/register", s.bearer(http.HandlerFunc(s.handleRegisterExtension)))
	mux.Handle("POST /v1/extensions/heartbeat", s.bearer(http.HandlerFunc(s.handleHeartbeatExtension)))
	mux.Handle("GET /v1/extensions", s.bearer(http.HandlerFunc(s.handleListExtensions)))

	// Web routes
	mux.HandleFunc("GET /{$}", s.handleHome)
	mux.HandleFunc("GET /conversations/{id}", s.handleConversation)
	mux.HandleFunc("POST /conversations/{id}/turns", s.handleWebPostTurn(runCtx))
	mux.HandleFunc("GET /turns/{id}", s.handleWebTurn)
	mux.HandleFunc("GET /settings", s.handleSettings)
	mux.HandleFunc("GET /settings/general", s.handleSettingsGeneral)
	mux.HandleFunc("POST /settings/general", s.handleSettingsSave)
	mux.HandleFunc("GET /settings/about", s.handleSettingsAbout)
	mux.HandleFunc("GET /settings/extensions", s.handleExtensions)
	mux.HandleFunc("GET /settings/extensions/add", s.handleExtensionAdd)
	mux.HandleFunc("POST /settings/extensions/add/url", s.handleExtensionAddURL)
	mux.HandleFunc("POST /settings/extensions/add/archive", s.handleExtensionAddArchive)
	mux.HandleFunc("GET /settings/extensions/{name}", s.handleExtension)
	mux.HandleFunc("POST /settings/extensions/{name}/remove", s.handleExtensionRemove)

	return mux
}
