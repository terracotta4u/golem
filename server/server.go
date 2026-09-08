package server

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/terracotta4u/golem/agent"
	"github.com/terracotta4u/golem/store"
)

//go:embed web/templates/*.html web/static
var webFS embed.FS

const maxBody = 1 << 20

type Options struct {
	Agent *agent.Agent
	Store store.Store
	Addr  string
	Token string

	StartExtension func(name string) error
	StopExtension  func(name string) error
}

type Server struct {
	opts Options
	tmpl *template.Template

	mu    sync.Mutex
	locks map[string]*sync.Mutex
	turns map[string]*turn
}

func New(opts Options) *Server {
	return &Server{
		opts:  opts,
		tmpl:  parseWeb(),
		locks: make(map[string]*sync.Mutex),
		turns: make(map[string]*turn),
	}
}

func NewToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func (s *Server) Handler() http.Handler {
	return s.handlerWith(context.Background())
}

func (s *Server) handler() http.Handler {
	return s.Handler()
}

func (s *Server) handlerWith(runCtx context.Context) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", s.handleHealth)
	s.mountStatic(mux)
	// API endpoints
	s.mountChat(mux, runCtx)
	// Web endpoints
	s.mountWebChat(mux, runCtx)
	s.mountWebSettings(mux)
	s.mountWebExtensions(mux)
	return mux
}

func parseWeb() *template.Template {
	return template.Must(template.ParseFS(webFS, "web/templates/*.html"))
}

func (s *Server) mountStatic(mux *http.ServeMux) {
	static, err := fs.Sub(webFS, "web/static")
	if err != nil {
		panic(err)
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(static))))
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) Listen(ctx context.Context, ready func()) error {
	ln, err := net.Listen("tcp", s.opts.Addr)
	if err != nil {
		return err
	}

	httpSrv := &http.Server{
		Handler:           s.handlerWith(ctx),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutCtx)
	}()

	fmt.Fprintf(os.Stderr, "golem listening on http://%s\n", ln.Addr())
	if ready != nil {
		ready()
	}
	err = httpSrv.Serve(ln)
	if err == http.ErrServerClosed {
		return ctx.Err()
	}
	return err
}

func (s *Server) startExtension(name string) error {
	if s.opts.StartExtension == nil {
		return nil
	}
	return s.opts.StartExtension(name)
}

func (s *Server) stopExtension(name string) error {
	if s.opts.StopExtension == nil {
		return nil
	}
	return s.opts.StopExtension(name)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) authorized(r *http.Request) bool {
	if s.opts.Token == "" {
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
	return subtle.ConstantTimeCompare([]byte(got), []byte(s.opts.Token)) == 1
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
