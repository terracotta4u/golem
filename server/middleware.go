package server

import (
	"crypto/subtle"
	"net"
	"net/http"
	"strings"
)

// localRequests keeps DNS-rebound hosts out of the local UI and rejects
// cross-origin browser mutations. Non-browser extension clients still use
// bearer authentication on /v1 routes.
func localRequests(next http.Handler) http.Handler {
	protected := http.NewCrossOriginProtection().Handler(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'")
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		} else if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
			host = host[1 : len(host)-1]
		}
		if !loopbackHost(host) {
			http.Error(w, "a loopback Host is required", http.StatusForbidden)
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
