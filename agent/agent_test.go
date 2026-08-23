package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/terracotta4u/golem/provider"
	"github.com/terracotta4u/golem/store"
	"github.com/terracotta4u/golem/tool"
)

func TestSendRunsToolThenReplies(t *testing.T) {
	echo := &stubTool{name: "echo", result: "pong"}
	p := &scriptedProvider{replies: []provider.Message{
		{
			Role: "assistant",
			ToolCalls: []provider.ToolCall{{
				ID:   "call_1",
				Type: "function",
				Function: provider.FunctionCall{
					Name:      "echo",
					Arguments: `{"text":"hi"}`,
				},
			}},
		},
		{Role: "assistant", Content: "done"},
	}}

	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	conv := store.New("cli")
	reply, err := New(p, echo).Session(st, conv).Send(context.Background(), "hello")
	if err != nil {
		t.Fatal(err)
	}
	if reply != "done" {
		t.Errorf("reply = %q, want done", reply)
	}
	if len(echo.calls) != 1 || echo.calls[0] != `{"text":"hi"}` {
		t.Errorf("tool calls = %v", echo.calls)
	}

	saved, err := st.Load(conv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Title != "hello" {
		t.Errorf("title = %q, want hello", saved.Title)
	}
	if len(saved.Messages) != 4 {
		t.Fatalf("messages = %d, want 4 (user, assistant tool call, tool, assistant)", len(saved.Messages))
	}
	if saved.Messages[2].Role != "tool" || saved.Messages[2].Content != "pong" || saved.Messages[2].ToolCallID != "call_1" {
		t.Errorf("tool message = %+v", saved.Messages[2])
	}
}

func TestUnknownToolIsMessage(t *testing.T) {
	p := &scriptedProvider{replies: []provider.Message{
		{
			Role: "assistant",
			ToolCalls: []provider.ToolCall{{
				ID:   "call_1",
				Type: "function",
				Function: provider.FunctionCall{
					Name:      "missing",
					Arguments: `{}`,
				},
			}},
		},
		{Role: "assistant", Content: "ok"},
	}}

	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	conv := store.New("cli")
	reply, err := New(p).Session(st, conv).Send(context.Background(), "hello")
	if err != nil {
		t.Fatal(err)
	}
	if reply != "ok" {
		t.Errorf("reply = %q, want ok (unknown tool should not fail the turn)", reply)
	}

	saved, err := st.Load(conv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Messages) < 3 || saved.Messages[2].Content != "unknown tool: missing" {
		t.Errorf("tool result = %+v, want unknown tool: missing", saved.Messages)
	}
}

type stubTool struct {
	name   string
	result string
	calls  []string
}

func (s *stubTool) Spec() tool.Spec {
	return tool.Spec{Name: s.name, Description: "stub"}
}

func (s *stubTool) Call(_ context.Context, args json.RawMessage) (string, error) {
	s.calls = append(s.calls, string(args))
	return s.result, nil
}

type scriptedProvider struct {
	replies []provider.Message
	i       int
}

func (p *scriptedProvider) Chat(_ context.Context, _ provider.ChatRequest) (provider.Message, error) {
	if p.i >= len(p.replies) {
		return provider.Message{}, errUnexpectedChat
	}
	msg := p.replies[p.i]
	p.i++
	return msg, nil
}

var errUnexpectedChat = errString("unexpected Chat call")

type errString string

func (e errString) Error() string { return string(e) }
