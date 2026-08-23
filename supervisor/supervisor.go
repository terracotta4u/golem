package supervisor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type Channel struct {
	Name    string
	Command string
	Args    []string
	Env     map[string]string
}

type Options struct {
	URL         string
	Token       string
	ChannelsDir string
	Channels    []Channel
}

type Supervisor struct {
	opts Options
	wg   sync.WaitGroup
}

func New(opts Options) *Supervisor {
	return &Supervisor{opts: opts}
}

func (s *Supervisor) Start(ctx context.Context) {
	for _, ch := range s.opts.Channels {
		s.wg.Add(1)
		go func(ch Channel) {
			defer s.wg.Done()
			_ = s.runOnce(ctx, ch)
		}(ch)
	}
}

func (s *Supervisor) Wait() {
	s.wg.Wait()
}

func (s *Supervisor) runOnce(ctx context.Context, ch Channel) error {
	bin, err := s.resolve(ch)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, bin, ch.Args...)
	cmd.Env = childEnv(s.opts.URL, s.opts.Token, ch.Env)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (s *Supervisor) resolve(ch Channel) (string, error) {
	command := strings.TrimSpace(ch.Command)
	if command == "" {
		command = ch.Name
	}
	if filepath.IsAbs(command) || strings.ContainsRune(command, filepath.Separator) {
		return command, nil
	}
	if s.opts.ChannelsDir != "" {
		local := filepath.Join(s.opts.ChannelsDir, command)
		if _, err := os.Stat(local); err == nil {
			return local, nil
		}
	}
	path, err := exec.LookPath(command)
	if err != nil {
		return "", fmt.Errorf("command %q not found in %s or PATH", command, s.opts.ChannelsDir)
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
