package server

import (
	"crypto/subtle"
	"net"
	"net/http"
	"strings"
)

// localRequests keeps unlisted hosts out and rejects cross-origin browser
// mutations. Non-browser extension clients still use bearer authentication
// on /v1 routes.
func (s *Server) localRequests(next http.Handler) http.Handler {
	protected := http.NewCrossOriginProtection().Handler(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.accessErr != nil {
			http.Error(w, s.accessErr.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'")
		if !s.access.allows(r.Host) {
			msg := "a loopback Host is required"
			if s.access.remote {
				msg = "this host is not allowed"
			}
			http.Error(w, msg, http.StatusForbidden)
			return
		}
		protected.ServeHTTP(w, r)
	})
}

func loopbackHost(host string) bool {
	return strings.EqualFold(host, "localhost") || net.ParseIP(host).IsLoopback()
}

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
