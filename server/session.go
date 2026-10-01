package server

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	sessionCookie   = "golem_session"
	sessionLifetime = 7 * 24 * time.Hour
)

type sessions struct {
	mu  sync.Mutex
	ids map[string]time.Time
}

func (s *sessions) issue() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	id := hex.EncodeToString(buf)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ids == nil {
		s.ids = make(map[string]time.Time)
	}
	s.ids[id] = time.Now().Add(sessionLifetime)
	return id, nil
}

func (s *sessions) valid(id string) bool {
	if id == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.ids[id]
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		delete(s.ids, id)
		return false
	}
	return true
}

func (s *sessions) drop(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.ids, id)
}

func secretEqual(want, got string) bool {
	if want == "" || got == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func (s *Server) sessionValid(r *http.Request) bool {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	return s.sessions.valid(c.Value)
}

func (s *Server) issueSession(w http.ResponseWriter) error {
	id, err := s.sessions.issue()
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    id,
		Path:     "/",
		MaxAge:   int(sessionLifetime.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   s.access.secure,
	})
	return nil
}

// web requires a session once the server is reachable beyond loopback.
// Local-only servers keep the passwordless UI.
func (s *Server) web(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.access.remote || s.sessionValid(r) {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		w.Header().Set("HX-Redirect", "/login")
		http.Error(w, "sign in required", http.StatusUnauthorized)
	})
}

func (s *Server) loginPage(w http.ResponseWriter, r *http.Request) {
	if !s.access.remote || s.sessionValid(r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.writeLogin(w, http.StatusOK, "")
}

func (s *Server) loginSubmit(w http.ResponseWriter, r *http.Request) {
	if !s.access.remote {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	if !secretEqual(s.opts.Token, strings.TrimSpace(r.FormValue("token"))) {
		s.writeLogin(w, http.StatusUnauthorized, "The token is incorrect.")
		return
	}
	if err := s.issueSession(w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		s.sessions.drop(c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   s.access.secure,
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) writeLogin(w http.ResponseWriter, status int, msg string) {
	body, err := s.pages.Execute("login", map[string]any{
		"Title": "Sign in",
		"Error": msg,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}
