package supervisor

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
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
}

func New(opts Options) *Supervisor {
	return &Supervisor{opts: opts}
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
	for _, ext := range s.opts.Extensions {
		s.wg.Add(1)
		go func(ext Extension) {
			defer s.wg.Done()
			s.keepAlive(ctx, ext)
		}(ext)
	}
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
	if bin := venvBin(ext.Dir); bin != "" {
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

func venvBin(dir string) string {
	if dir == "" {
		return ""
	}
	bin := filepath.Join(dir, ".venv", "bin")
	if runtime.GOOS == "windows" {
		bin = filepath.Join(dir, ".venv", "Scripts")
	}
	info, err := os.Stat(bin)
	if err != nil || !info.IsDir() {
		return ""
	}
	return bin
}
