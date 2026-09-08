package supervisor

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/terracotta4u/golem/extension"
)

const (
	minBackoff   = time.Second
	maxBackoff   = 30 * time.Second
	healthyAfter = time.Minute
)

type Extension struct {
	Name    string
	Command string
	Args    []string
	Env     map[string]string
	Dir     string
}

type Options struct {
	URL        string
	Token      string
	Extensions []Extension
}

type Supervisor struct {
	opts Options
	wg   sync.WaitGroup

	mu     sync.Mutex
	parent context.Context
	procs  map[string]*run
}

type run struct {
	cancel context.CancelFunc
	done   chan struct{}
}

func New(opts Options) *Supervisor {
	return &Supervisor{
		opts:  opts,
		procs: make(map[string]*run),
	}
}

func URLFromListen(listen string) string {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return "http://" + listen
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port)
}

func (s *Supervisor) Start(ctx context.Context) {
	s.mu.Lock()
	s.parent = ctx
	s.mu.Unlock()
	for _, ext := range s.opts.Extensions {
		if err := s.Add(ext); err != nil {
			fmt.Fprintf(os.Stderr, "supervisor: start %s: %v\n", ext.Name, err)
		}
	}
}

func (s *Supervisor) Add(ext Extension) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.parent == nil {
		return fmt.Errorf("supervisor: not started")
	}
	if err := s.parent.Err(); err != nil {
		return err
	}
	name := strings.TrimSpace(ext.Name)
	if name == "" {
		return fmt.Errorf("supervisor: missing extension name")
	}
	if _, ok := s.procs[name]; ok {
		return fmt.Errorf("supervisor: extension %s already running", name)
	}

	ctx, cancel := context.WithCancel(s.parent)
	done := make(chan struct{})
	s.procs[name] = &run{cancel: cancel, done: done}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer close(done)
		s.keepAlive(ctx, ext)
	}()
	return nil
}

func (s *Supervisor) Stop(name string) error {
	s.mu.Lock()
	r, ok := s.procs[name]
	s.mu.Unlock()
	if !ok {
		return nil
	}
	r.cancel()
	<-r.done
	s.mu.Lock()
	delete(s.procs, name)
	s.mu.Unlock()
	return nil
}

func (s *Supervisor) Wait() {
	s.wg.Wait()
}

func (s *Supervisor) keepAlive(ctx context.Context, ext Extension) {
	backoff := minBackoff
	for {
		if ctx.Err() != nil {
			return
		}

		started := time.Now()
		err := s.runOnce(ctx, ext)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "extension %s: %v\n", ext.Name, err)
		} else {
			fmt.Fprintf(os.Stderr, "extension %s: exited\n", ext.Name)
		}

		if time.Since(started) > healthyAfter {
			backoff = minBackoff
		}
		fmt.Fprintf(os.Stderr, "extension %s: restart in %s\n", ext.Name, backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func (s *Supervisor) runOnce(ctx context.Context, ext Extension) error {
	command := strings.TrimSpace(ext.Command)
	if command == "" {
		return fmt.Errorf("extension %s: missing command", ext.Name)
	}

	cmd := exec.CommandContext(ctx, command, ext.Args...)
	cmd.Env = childEnv(s.opts.URL, s.opts.Token, ext)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if ext.Dir != "" {
		cmd.Dir = ext.Dir
	}
	return cmd.Run()
}

func childEnv(url, token string, ext Extension) []string {
	parent := os.Environ()
	out := make([]string, 0, len(parent)+4)
	path := ""
	for _, kv := range parent {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		if strings.EqualFold(k, "PATH") {
			path = v
			continue
		}
		out = append(out, kv)
	}
	if bin := extension.VenvBin(ext.Dir); bin != "" {
		if path != "" {
			path = bin + string(os.PathListSeparator) + path
		} else {
			path = bin
		}
	}
	if path != "" {
		out = append(out, "PATH="+path)
	}
	out = append(out, "GOLEM_URL="+url)
	if token != "" {
		out = append(out, "GOLEM_TOKEN="+token)
	}
	for k, v := range ext.Env {
		if strings.TrimSpace(v) == "" {
			continue
		}
		out = append(out, k+"="+v)
	}
	return out
}
