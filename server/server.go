package server

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/terracotta4u/golem/agent"
	"github.com/terracotta4u/golem/store"
)

const maxBody = 1 << 20

type Options struct {
	Agent *agent.Agent
	Store store.Store
	Addr  string
	Token string
}

type Server struct {
	opts Options

	mu    sync.Mutex
	locks map[string]*sync.Mutex
	turns map[string]*turn
}

func New(opts Options) *Server {
	return &Server{
		opts:  opts,
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
	s.mountChat(mux, runCtx)
	return mux
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
