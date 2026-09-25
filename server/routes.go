package server

import (
	"context"
	"net/http"
)

func (s *Server) routes(runCtx context.Context) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /static/", s.static())

	// API routes
	mux.HandleFunc("GET /v1/health", s.handleHealth)
	mux.HandleFunc("POST /v1/conversations/{id}/turns", s.handlePostTurn(runCtx))
	mux.HandleFunc("GET /v1/turns/{id}", s.handleGetTurn)
	mux.HandleFunc("POST /v1/extensions/register", s.handleRegisterExtension)
	mux.HandleFunc("POST /v1/extensions/heartbeat", s.handleHeartbeatExtension)
	mux.HandleFunc("GET /v1/extensions", s.handleListExtensions)

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
