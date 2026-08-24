package agent

import (
	"context"
	"encoding/json"
	"testing"

	"strings"

	"github.com/terracotta4u/golem/provider"
	"github.com/terracotta4u/golem/skill"
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
	reply, err := New(p, nil, echo).Session(st, conv).Send(context.Background(), "hello")
	if err != nil {
		t.Fatal(err)
	}
	if reply != "done" {
		t.Errorf("reply = %q, want done", reply)
	}
	if len(echo.calls) != 1 || echo.calls[0] != `{"text":"hi"}` {
		t.Errorf("tool calls = %v", echo.calls)
	}
	if len(p.got) == 0 {
		t.Fatal("no Chat calls")
	}
	for i, req := range p.got {
		if req.Messages[0].Role != "system" || req.Messages[0].Content != systemPrompt(nil) {
			t.Errorf("chat %d first message = %+v, want system prompt", i, req.Messages[0])
		}
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
	reply, err := New(p, nil).Session(st, conv).Send(context.Background(), "hello")
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

func TestSendLoadsSkillIntoPrompt(t *testing.T) {
	dir := t.TempDir()
	sk := skill.Skill{
		Name:        "commit",
		Description: "Write commit messages.",
		Body:        "Follow the commit format.",
		Dir:         dir,
	}
	p := &scriptedProvider{replies: []provider.Message{
		{
			Role: "assistant",
			ToolCalls: []provider.ToolCall{{
				ID:   "call_1",
				Type: "function",
				Function: provider.FunctionCall{
					Name:      "skill",
					Arguments: `{"name":"commit"}`,
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
	reply, err := New(p, []skill.Skill{sk}).Session(st, conv).Send(context.Background(), "hello")
	if err != nil {
		t.Fatal(err)
	}
	if reply != "done" {
		t.Errorf("reply = %q, want done", reply)
	}
	if len(p.got) == 0 {
		t.Fatal("no Chat calls")
	}
	for i, req := range p.got {
		sys := req.Messages[0].Content
		if !strings.Contains(sys, "- commit: Write commit messages.") {
			t.Errorf("chat %d system = %q, want skill catalog", i, sys)
		}
		found := false
		for _, def := range req.Tools {
			if def.Name == "skill" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("chat %d tools = %v, want skill", i, req.Tools)
		}
	}

	saved, err := st.Load(conv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Messages) < 3 || !strings.Contains(saved.Messages[2].Content, "Follow the commit format.") {
		t.Errorf("skill result = %+v, want body", saved.Messages)
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
	got     []provider.ChatRequest
	i       int
}

func (p *scriptedProvider) Chat(_ context.Context, req provider.ChatRequest) (provider.Message, error) {
	p.got = append(p.got, req)
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
