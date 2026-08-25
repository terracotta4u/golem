package supervisor

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
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
	bin, err := s.resolve(ext)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, bin, ext.Args...)
	cmd.Env = childEnv(s.opts.URL, s.opts.Token, ext.Env)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if ext.Dir != "" {
		cmd.Dir = ext.Dir
	}
	return cmd.Run()
}

func (s *Supervisor) resolve(ext Extension) (string, error) {
	command := strings.TrimSpace(ext.Command)
	if command == "" {
		command = ext.Name
	}
	if filepath.IsAbs(command) {
		return command, nil
	}
	if ext.Dir != "" {
		local := filepath.Join(ext.Dir, command)
		if _, err := os.Stat(local); err == nil {
			return local, nil
		}
		if strings.ContainsRune(command, filepath.Separator) {
			return "", fmt.Errorf("command %q not found in %s", command, ext.Dir)
		}
	}
	path, err := exec.LookPath(command)
	if err != nil {
		if ext.Dir != "" {
			return "", fmt.Errorf("command %q not found in %s or PATH", command, ext.Dir)
		}
		return "", fmt.Errorf("command %q not found in PATH", command)
	}
	return path, nil
}

func childEnv(url, token string, extra map[string]string) []string {
	env := os.Environ()
	env = append(env, "GOLEM_URL="+url)
	if token != "" {
		env = append(env, "GOLEM_TOKEN="+token)
	}
	for k, v := range extra {
		env = append(env, k+"="+v)
	}
	return env
}
