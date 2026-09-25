package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/terracotta4u/golem/agent"
	"github.com/terracotta4u/golem/conversation"
	"github.com/terracotta4u/golem/registry"
	"github.com/terracotta4u/golem/release"
	"github.com/terracotta4u/golem/server/api"
	"github.com/terracotta4u/golem/server/web"
)

const maxBody = 1 << 20

type Options struct {
	Agent *agent.Agent
	Store conversation.Store
	Addr  string
	Token string

	StartExtension func(name string) error
	StopExtension  func(name string) error

	Registry *registry.Registry

	Version string
	Release *release.Checker
}

type Server struct {
	opts       Options
	pages      *web.Pages
	settings   *web.Settings
	extensions *web.Extensions
	regAPI     *api.Extensions
	reg        *registry.Registry

	mu    sync.Mutex
	locks map[string]*sync.Mutex
	turns map[string]*turn

	updateOnce sync.Once
	update     release.Status
	updateOK   bool
}

func New(opts Options) *Server {
	s := &Server{
		opts:  opts,
		pages: web.New(),
		locks: make(map[string]*sync.Mutex),
		turns: make(map[string]*turn),
	}
	if opts.Registry != nil {
		s.reg = opts.Registry
		s.reg.SetToken(opts.Token)
	} else {
		s.reg = registry.New(opts.Token)
	}
	if s.opts.Agent != nil && s.opts.Agent.Catalog == nil {
		s.opts.Agent.Catalog = s.reg
	}
	s.settings = web.NewSettings(s.pages, opts.Version, s.updateStatus)
	s.extensions = web.NewExtensions(s.pages, s.startExtension, s.stopExtension)
	s.regAPI = api.NewExtensions(s.reg)
	return s
}

func NewToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func (s *Server) Handler() http.Handler {
	return s.routes(context.Background())
}

func (s *Server) handler() http.Handler {
	return s.Handler()
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	s.pages.Render(w, name, data)
}

func (s *Server) execute(name string, data any) (string, error) {
	return s.pages.Execute(name, data)
}

func (s *Server) Listen(ctx context.Context, ready func()) error {
	ln, err := net.Listen("tcp", s.opts.Addr)
	if err != nil {
		return err
	}

	httpSrv := &http.Server{
		Handler:           s.routes(ctx),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutCtx)
	}()

	fmt.Fprintf(os.Stderr, "Golem listening on http://%s\n\n", ln.Addr())
	go s.reportUpdate(ctx, os.Stderr)
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
	s.reg.Drop(name)
	if s.opts.StopExtension == nil {
		return nil
	}
	return s.opts.StopExtension(name)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
