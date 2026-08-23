package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/terracotta4u/golem/agent"
	"github.com/terracotta4u/golem/config"
	"github.com/terracotta4u/golem/provider"
	"github.com/terracotta4u/golem/server"
	"github.com/terracotta4u/golem/store"
)

func TestChatSendsWhenRunning(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	dir, err := config.Dir()
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(server.New(server.Options{
		Agent: agent.New(&replyProvider{text: "hello back"}),
		Store: st,
		Token: "secret",
	}).Handler())
	defer ts.Close()

	path, err := config.DaemonPath()
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(instanceState{URL: ts.URL, Token: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldIn, oldOut := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = stdinR, stdoutW
	defer func() {
		os.Stdin, os.Stdout = oldIn, oldOut
	}()

	go func() {
		stdinW.WriteString("hello\n/quit\n")
		stdinW.Close()
	}()

	err = runChat()
	stdoutW.Close()
	out, _ := io.ReadAll(stdoutR)
	stdinR.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte("hello back")) {
		t.Fatalf("stdout = %q, want hello back", out)
	}
}

type replyProvider struct {
	text string
}

func (p *replyProvider) Chat(_ context.Context, _ provider.ChatRequest) (provider.Message, error) {
	return provider.Message{Role: "assistant", Content: p.text}, nil
}
