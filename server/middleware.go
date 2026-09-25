package server

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

func (s *Server) bearer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !authorized(s.opts.Token, r) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func authorized(token string, r *http.Request) bool {
	if token == "" {
		return true
	}
	const prefix = "Bearer "
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, prefix) {
		return false
	}
	got := strings.TrimSpace(strings.TrimPrefix(auth, prefix))
	if got == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1
}
