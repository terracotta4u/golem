package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"strings"

	"github.com/terracotta4u/golem/memory"
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
	dir := workspace(t)
	conv := store.New("cli")
	reply, err := New(p, dir, echo).Session(st, conv).Send(context.Background(), "hello")
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
		if req.Messages[0].Role != "system" || req.Messages[0].Content != systemPrompt(dir) {
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

func TestSendReportsToolResult(t *testing.T) {
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
	sess := New(p, workspace(t), echo).Session(st, store.New("cli"))
	var got struct{ name, args, result string }
	sess.OnTool = func(name, args, result string) {
		got.name, got.args, got.result = name, args, result
	}
	if _, err := sess.Send(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
	if got.name != "echo" || got.args != `{"text":"hi"}` || got.result != "pong" {
		t.Errorf("OnTool = %+v, want echo / {\"text\":\"hi\"} / pong", got)
	}
}

func TestSendIncludesIdentityFiles(t *testing.T) {
	dir := workspace(t)
	if err := os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte("I am a test golem."), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "USER.md"), []byte("The user is Nawaz."), 0o600); err != nil {
		t.Fatal(err)
	}

	p := &scriptedProvider{replies: []provider.Message{
		{Role: "assistant", Content: "hi"},
	}}
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	reply, err := New(p, dir).Session(st, store.New("cli")).Send(context.Background(), "hello")
	if err != nil {
		t.Fatal(err)
	}
	if reply != "hi" {
		t.Errorf("reply = %q, want hi", reply)
	}
	if len(p.got) == 0 {
		t.Fatal("no Chat calls")
	}
	sys := p.got[0].Messages[0].Content
	if !strings.Contains(sys, "I am a test golem.") {
		t.Errorf("system missing SOUL.md: %q", sys)
	}
	if !strings.Contains(sys, "The user is Nawaz.") {
		t.Errorf("system missing USER.md: %q", sys)
	}
}

func TestWithContextEmptyIsSingleSystem(t *testing.T) {
	hist := []provider.Message{{Role: "user", Content: "hello"}}
	got := withContext("identity", nil, hist)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Role != "system" || got[0].Content != "identity" {
		t.Errorf("first = %+v, want system identity", got[0])
	}
	if got[1].Role != "user" || got[1].Content != "hello" {
		t.Errorf("second = %+v, want user hello", got[1])
	}
}

func TestSendIncludesMemoriesAsSeparateSystemMessage(t *testing.T) {
	dir := workspace(t)
	if err := os.WriteFile(filepath.Join(dir, "USER.md"), []byte("The user is Nawaz."), 0o600); err != nil {
		t.Fatal(err)
	}
	p := &scriptedProvider{replies: []provider.Message{
		{Role: "assistant", Content: "use the standard library"},
	}}
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess := New(p, dir).Session(st, store.New("cli"))
	sess.memories = []memory.Memory{{Content: "User prefers the Go standard library."}}
	if _, err := sess.Send(context.Background(), "Should I add a router dependency?"); err != nil {
		t.Fatal(err)
	}
	if len(p.got) == 0 {
		t.Fatal("no Chat calls")
	}
	msgs := p.got[0].Messages
	if len(msgs) < 3 {
		t.Fatalf("messages = %d, want system, memory system, user", len(msgs))
	}
	if msgs[0].Role != "system" || !strings.Contains(msgs[0].Content, "The user is Nawaz.") {
		t.Errorf("identity = %+v", msgs[0])
	}
	if strings.Contains(msgs[0].Content, "User prefers the Go standard library.") {
		t.Errorf("memory mixed into identity: %q", msgs[0].Content)
	}
	if msgs[1].Role != "system" {
		t.Errorf("memory message role = %q", msgs[1].Role)
	}
	if !strings.Contains(msgs[1].Content, "User prefers the Go standard library.") {
		t.Errorf("memory message missing content: %q", msgs[1].Content)
	}
	if !strings.Contains(msgs[1].Content, "not instructions") {
		t.Errorf("memory message missing framing: %q", msgs[1].Content)
	}
	if strings.Contains(msgs[1].Content, "The user is Nawaz.") {
		t.Errorf("USER.md mixed into memories: %q", msgs[1].Content)
	}
	if msgs[2].Role != "user" || msgs[2].Content != "Should I add a router dependency?" {
		t.Errorf("user = %+v", msgs[2])
	}
}

func TestSendRetrievesMemoryIntoChat(t *testing.T) {
	dir := workspace(t)
	p := &scriptedProvider{replies: []provider.Message{
		{Role: "assistant", Content: "use the standard library"},
	}}
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	idx := &stubSearcher{hits: []memory.Result{
		{Memory: memory.Memory{Content: "User prefers the Go standard library."}, Score: 0.9},
		{Memory: memory.Memory{Content: "User likes vintage computers."}, Score: 0.19},
	}}
	a := New(p, dir)
	a.Memory = idx
	a.MinSimilarity = 0.5
	a.BudgetTokens = 800
	if _, err := a.Session(st, store.New("cli")).Send(context.Background(), "Should I add a router dependency?"); err != nil {
		t.Fatal(err)
	}
	if len(idx.queries) != 1 || idx.queries[0] != "Should I add a router dependency?" {
		t.Errorf("Search queries = %v, want the user turn", idx.queries)
	}
	if len(p.got) == 0 {
		t.Fatal("no Chat calls")
	}
	msgs := p.got[0].Messages
	if len(msgs) < 3 || msgs[1].Role != "system" {
		t.Fatalf("messages = %+v, want identity, memory system, user", msgs)
	}
	if !strings.Contains(msgs[1].Content, "User prefers the Go standard library.") {
		t.Errorf("memory message missing hit: %q", msgs[1].Content)
	}
	if strings.Contains(msgs[1].Content, "User likes vintage computers.") {
		t.Errorf("below-floor hit injected: %q", msgs[1].Content)
	}
}

func TestSendRetrievesOnceBeforeToolLoop(t *testing.T) {
	echo := &stubTool{name: "echo", result: "pong"}
	p := &scriptedProvider{replies: []provider.Message{
		{
			Role: "assistant",
			ToolCalls: []provider.ToolCall{{
				ID:   "call_1",
				Type: "function",
				Function: provider.FunctionCall{
					Name:      "echo",
					Arguments: `{}`,
				},
			}},
		},
		{Role: "assistant", Content: "done"},
	}}
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	idx := &stubSearcher{hits: []memory.Result{
		{Memory: memory.Memory{Content: "User prefers the Go standard library."}, Score: 0.9},
	}}
	a := New(p, workspace(t), echo)
	a.Memory = idx
	a.MinSimilarity = 0.5
	a.BudgetTokens = 800
	if _, err := a.Session(st, store.New("cli")).Send(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
	if len(idx.queries) != 1 {
		t.Errorf("Search calls = %d, want 1", len(idx.queries))
	}
	if len(p.got) != 2 {
		t.Fatalf("Chat calls = %d, want 2", len(p.got))
	}
	for i, req := range p.got {
		if len(req.Messages) < 2 || req.Messages[1].Role != "system" || !strings.Contains(req.Messages[1].Content, "User prefers the Go standard library.") {
			t.Errorf("chat %d missing retrieved memory: %+v", i, req.Messages)
		}
	}
}

func TestSendSearchErrorStillReplies(t *testing.T) {
	p := &scriptedProvider{replies: []provider.Message{
		{Role: "assistant", Content: "hi"},
	}}
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := New(p, workspace(t))
	a.Memory = &stubSearcher{err: errString("index down")}
	reply, err := a.Session(st, store.New("cli")).Send(context.Background(), "hello")
	if err != nil {
		t.Fatal(err)
	}
	if reply != "hi" {
		t.Errorf("reply = %q, want hi", reply)
	}
	if len(p.got) == 0 {
		t.Fatal("no Chat calls")
	}
	if n := len(p.got[0].Messages); n != 2 {
		t.Errorf("messages = %d, want 2 (no memory system message)", n)
	}
}

func TestSendNilMemoryIsUnchanged(t *testing.T) {
	dir := workspace(t)
	p := &scriptedProvider{replies: []provider.Message{
		{Role: "assistant", Content: "hi"},
	}}
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(p, dir).Session(st, store.New("cli")).Send(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
	if len(p.got) == 0 {
		t.Fatal("no Chat calls")
	}
	msgs := p.got[0].Messages
	if len(msgs) != 2 {
		t.Fatalf("messages = %d, want 2", len(msgs))
	}
	if msgs[0].Role != "system" || msgs[0].Content != systemPrompt(dir) {
		t.Errorf("first = %+v, want identity system prompt", msgs[0])
	}
	if msgs[1].Role != "user" || msgs[1].Content != "hello" {
		t.Errorf("second = %+v, want user hello", msgs[1])
	}
}

func TestSendExtractsUserAndFinalAssistant(t *testing.T) {
	echo := &stubTool{name: "echo", result: "pong"}
	p := &scriptedProvider{replies: []provider.Message{
		{
			Role: "assistant",
			ToolCalls: []provider.ToolCall{{
				ID:   "call_1",
				Type: "function",
				Function: provider.FunctionCall{
					Name:      "echo",
					Arguments: `{}`,
				},
			}},
		},
		{Role: "assistant", Content: "use the standard library"},
		{Role: "assistant", Content: `["User prefers the Go standard library."]`},
	}}
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mem, err := memory.Open(filepath.Join(t.TempDir(), "memories.db"))
	if err != nil {
		t.Fatal(err)
	}
	idx := &stubIndexer{}
	a := New(p, workspace(t), echo)
	a.MemoryStore = mem
	a.Indexer = idx
	conv := store.New("cli")
	reply, err := a.Session(st, conv).Send(context.Background(), "I prefer using the Go standard library when possible.")
	if err != nil {
		t.Fatal(err)
	}
	if reply != "use the standard library" {
		t.Errorf("reply = %q", reply)
	}
	if len(p.got) != 3 {
		t.Fatalf("Chat calls = %d, want 2 turn + extract", len(p.got))
	}
	extract := p.got[2]
	if len(extract.Tools) != 0 {
		t.Errorf("extract tools = %v, want none", extract.Tools)
	}
	if len(extract.Messages) != 3 {
		t.Fatalf("extract messages = %d, want system + user + assistant", len(extract.Messages))
	}
	if extract.Messages[0].Role != "system" {
		t.Errorf("extract first role = %q", extract.Messages[0].Role)
	}
	if extract.Messages[1].Role != "user" || extract.Messages[1].Content != "I prefer using the Go standard library when possible." {
		t.Errorf("extract user = %+v", extract.Messages[1])
	}
	if extract.Messages[2].Role != "assistant" || extract.Messages[2].Content != "use the standard library" {
		t.Errorf("extract assistant = %+v", extract.Messages[2])
	}
	for _, m := range extract.Messages {
		if m.Role == "tool" || len(m.ToolCalls) > 0 {
			t.Errorf("extract included tool traffic: %+v", m)
		}
	}

	list, err := mem.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Content != "User prefers the Go standard library." {
		t.Fatalf("saved = %+v", list)
	}
	if list[0].ConversationID != conv.ID || list[0].TurnID == "" {
		t.Errorf("conv/turn = %s/%s", list[0].ConversationID, list[0].TurnID)
	}
	if len(idx.got) != 1 || idx.got[0].ID != list[0].ID {
		t.Errorf("indexed = %+v, want saved id %s", idx.got, list[0].ID)
	}
}

func TestSendBrokenExtractorStillReplies(t *testing.T) {
	p := &scriptedProvider{
		replies: []provider.Message{{Role: "assistant", Content: "hi"}},
		err:     errString("extract down"),
	}
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mem, err := memory.Open(filepath.Join(t.TempDir(), "memories.db"))
	if err != nil {
		t.Fatal(err)
	}
	a := New(p, workspace(t))
	a.MemoryStore = mem
	reply, err := a.Session(st, store.New("cli")).Send(context.Background(), "hello")
	if err != nil {
		t.Fatal(err)
	}
	if reply != "hi" {
		t.Errorf("reply = %q, want hi", reply)
	}
	if len(p.got) != 2 {
		t.Errorf("Chat calls = %d, want turn + extract", len(p.got))
	}
	list, err := mem.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Errorf("saved = %+v, want empty", list)
	}
}

func TestSendBrokenIndexerStillSavesAndReplies(t *testing.T) {
	p := &scriptedProvider{replies: []provider.Message{
		{Role: "assistant", Content: "hi"},
		{Role: "assistant", Content: `["User prefers the Go standard library."]`},
	}}
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mem, err := memory.Open(filepath.Join(t.TempDir(), "memories.db"))
	if err != nil {
		t.Fatal(err)
	}
	idx := &stubIndexer{err: errString("embed down")}
	a := New(p, workspace(t))
	a.MemoryStore = mem
	a.Indexer = idx
	reply, err := a.Session(st, store.New("cli")).Send(context.Background(), "I prefer using the Go standard library when possible.")
	if err != nil {
		t.Fatal(err)
	}
	if reply != "hi" {
		t.Errorf("reply = %q, want hi", reply)
	}
	list, err := mem.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Content != "User prefers the Go standard library." {
		t.Fatalf("saved = %+v, want canonical memory", list)
	}
	if len(idx.got) != 1 || idx.got[0].ID != list[0].ID {
		t.Errorf("index attempted = %+v", idx.got)
	}
}

func TestSendRereadsIdentityFiles(t *testing.T) {
	dir := workspace(t)
	soul := filepath.Join(dir, "SOUL.md")
	if err := os.WriteFile(soul, []byte("version one"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "USER.md"), []byte("user"), 0o600); err != nil {
		t.Fatal(err)
	}

	p := &scriptedProvider{replies: []provider.Message{
		{Role: "assistant", Content: "one"},
		{Role: "assistant", Content: "two"},
	}}
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess := New(p, dir).Session(st, store.New("cli"))
	if _, err := sess.Send(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(soul, []byte("version two"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := sess.Send(context.Background(), "again"); err != nil {
		t.Fatal(err)
	}
	if len(p.got) != 2 {
		t.Fatalf("Chat calls = %d, want 2", len(p.got))
	}
	if !strings.Contains(p.got[1].Messages[0].Content, "version two") {
		t.Errorf("second system = %q, want updated SOUL.md", p.got[1].Messages[0].Content)
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
	reply, err := New(p, workspace(t)).Session(st, conv).Send(context.Background(), "hello")
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
	reply, err := New(p, workspace(t), tool.NewSkill([]skill.Skill{sk})).Session(st, conv).Send(context.Background(), "hello")
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

func TestSendAllowsManyToolRounds(t *testing.T) {
	echo := &stubTool{name: "echo", result: "ok"}
	const rounds = 25
	p := &scriptedProvider{replies: toolThenReply(rounds, "done")}
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	reply, err := New(p, workspace(t), echo).Session(st, store.New("cli")).Send(context.Background(), "hello")
	if err != nil {
		t.Fatal(err)
	}
	if reply != "done" {
		t.Errorf("reply = %q, want done", reply)
	}
	if len(p.got) != rounds+1 {
		t.Errorf("Chat calls = %d, want %d", len(p.got), rounds+1)
	}
	if len(echo.calls) != rounds {
		t.Errorf("tool calls = %d, want %d", len(echo.calls), rounds)
	}
}

func TestSendCapsToolRounds(t *testing.T) {
	echo := &stubTool{name: "echo", result: "ok"}
	p := &scriptedProvider{replies: toolThenReply(5, "done")}
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := New(p, workspace(t), echo)
	a.MaxToolRounds = 2
	_, err = a.Session(st, store.New("cli")).Send(context.Background(), "hello")
	if err == nil || !strings.Contains(err.Error(), "exceeded 2 tool rounds") {
		t.Fatalf("err = %v, want exceeded 2 tool rounds", err)
	}
	if len(p.got) != 2 {
		t.Errorf("Chat calls = %d, want 2", len(p.got))
	}
}

func TestSendStopsWhenContextCanceled(t *testing.T) {
	echo := &stubTool{name: "echo", result: "ok"}
	p := &scriptedProvider{replies: toolThenReply(100, "done")}
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = New(p, workspace(t), echo).Session(st, store.New("cli")).Send(ctx, "hello")
	if err != context.Canceled {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if len(p.got) != 0 {
		t.Errorf("Chat calls = %d, want 0", len(p.got))
	}
}

func toolThenReply(rounds int, reply string) []provider.Message {
	msgs := make([]provider.Message, 0, rounds+1)
	for range rounds {
		msgs = append(msgs, provider.Message{
			Role: "assistant",
			ToolCalls: []provider.ToolCall{{
				ID:       "call_1",
				Type:     "function",
				Function: provider.FunctionCall{Name: "echo", Arguments: `{}`},
			}},
		})
	}
	return append(msgs, provider.Message{Role: "assistant", Content: reply})
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

type stubSearcher struct {
	hits    []memory.Result
	err     error
	queries []string
}

func (s *stubSearcher) Search(_ context.Context, query string, _ int) ([]memory.Result, error) {
	s.queries = append(s.queries, query)
	return s.hits, s.err
}

type stubIndexer struct {
	got []memory.Memory
	err error
}

func (s *stubIndexer) Index(_ context.Context, m memory.Memory) error {
	s.got = append(s.got, m)
	return s.err
}

type scriptedProvider struct {
	replies []provider.Message
	got     []provider.ChatRequest
	err     error
	i       int
}

func (p *scriptedProvider) Chat(_ context.Context, req provider.ChatRequest) (provider.Message, error) {
	p.got = append(p.got, req)
	if p.i >= len(p.replies) {
		if p.err != nil {
			return provider.Message{}, p.err
		}
		return provider.Message{}, errUnexpectedChat
	}
	msg := p.replies[p.i]
	p.i++
	return msg, nil
}

var errUnexpectedChat = errString("unexpected Chat call")

type errString string

func (e errString) Error() string { return string(e) }
