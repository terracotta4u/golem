package memory

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/terracotta4u/golem/provider"
)

func TestExtractParsesJSONArray(t *testing.T) {
	p := &scriptedProvider{replies: []provider.Message{
		{Role: "assistant", Content: `["User is building Golem in Go.", "User values distributing Golem as a single binary."]`},
	}}
	turn := []provider.Message{
		{Role: "user", Content: "I'm building Golem in Go because I like being able to distribute a single binary."},
		{Role: "assistant", Content: "That makes sense for a personal agent."},
	}

	got, err := Extract(context.Background(), p, turn)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"User is building Golem in Go.",
		"User values distributing Golem as a single binary.",
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if len(p.got) != 1 {
		t.Fatalf("Chat calls = %d, want 1", len(p.got))
	}
	req := p.got[0]
	if len(req.Tools) != 0 {
		t.Errorf("tools = %v, want none", req.Tools)
	}
	if len(req.Messages) < 2 || req.Messages[0].Role != "system" {
		t.Fatalf("messages = %+v, want system then turn", req.Messages)
	}
	sys := req.Messages[0].Content
	if !strings.Contains(sys, "JSON") {
		t.Errorf("system missing JSON instruction: %q", sys)
	}
	if !strings.Contains(sys, "lasting") && !strings.Contains(sys, "durable") {
		t.Errorf("system missing durable/lasting instruction: %q", sys)
	}
	if len(req.Messages) != 1+len(turn) {
		t.Fatalf("messages = %d, want system + %d turn messages", len(req.Messages), len(turn))
	}
	for i, m := range turn {
		got := req.Messages[i+1]
		if got.Role != m.Role || got.Content != m.Content {
			t.Errorf("turn message %d = %+v, want %+v", i, got, m)
		}
	}
}

func TestExtractEmptyOrInvalidYieldsNothing(t *testing.T) {
	cases := []string{
		"",
		"[]",
		"not json",
		`{"memory":"nope"}`,
		"```json\n[]\n```",
	}
	for _, content := range cases {
		p := &scriptedProvider{replies: []provider.Message{
			{Role: "assistant", Content: content},
		}}
		got, err := Extract(context.Background(), p, []provider.Message{
			{Role: "user", Content: "hi"},
		})
		if err != nil {
			t.Errorf("content %q: err = %v, want nil", content, err)
		}
		if len(got) != 0 {
			t.Errorf("content %q: got %v, want empty", content, got)
		}
	}
}

func TestExtractStripsFencesAndBlanks(t *testing.T) {
	p := &scriptedProvider{replies: []provider.Message{
		{Role: "assistant", Content: "```json\n[\"User prefers the Go standard library.\", \"\", \"  \"]\n```"},
	}}
	got, err := Extract(context.Background(), p, []provider.Message{
		{Role: "user", Content: "I prefer using the Go standard library when possible."},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "User prefers the Go standard library." {
		t.Errorf("got %v", got)
	}
}

func TestExtractThenSave(t *testing.T) {
	p := &scriptedProvider{replies: []provider.Message{
		{Role: "assistant", Content: `["User is building Golem in Go.", "User values distributing Golem as a single binary."]`},
	}}
	contents, err := Extract(context.Background(), p, []provider.Message{
		{Role: "user", Content: "I'm building Golem in Go because I like being able to distribute a single binary."},
	})
	if err != nil {
		t.Fatal(err)
	}

	st, err := Open(filepath.Join(t.TempDir(), "memories.db"))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := st.SaveExtracted(contents, "conv-1", "turn-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(saved) != 2 {
		t.Fatalf("saved %d, want 2", len(saved))
	}

	list, err := st.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("list len = %d, want 2", len(list))
	}

	ids := map[string]bool{}
	seen := map[string]bool{}
	for _, m := range list {
		if m.ID == "" {
			t.Error("missing id")
		}
		if ids[m.ID] {
			t.Errorf("duplicate id %s", m.ID)
		}
		ids[m.ID] = true
		if m.ConversationID != "conv-1" || m.TurnID != "turn-1" {
			t.Errorf("got conv/turn %s/%s", m.ConversationID, m.TurnID)
		}
		if m.CreatedAt.IsZero() || m.CreatedAt.After(time.Now().UTC().Add(time.Second)) {
			t.Errorf("created_at = %v", m.CreatedAt)
		}
		seen[m.Content] = true
	}
	if !seen["User is building Golem in Go."] || !seen["User values distributing Golem as a single binary."] {
		t.Errorf("contents = %v", seen)
	}
}

func TestSaveExtractedEmpty(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "memories.db"))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := st.SaveExtracted(nil, "conv-1", "turn-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(saved) != 0 {
		t.Errorf("saved = %v, want empty", saved)
	}
	list, err := st.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Errorf("list = %v, want empty", list)
	}
}

func TestExtractProviderError(t *testing.T) {
	p := &scriptedProvider{err: errString("boom")}
	_, err := Extract(context.Background(), p, []provider.Message{
		{Role: "user", Content: "hi"},
	})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v, want boom", err)
	}
}

type scriptedProvider struct {
	replies []provider.Message
	got     []provider.ChatRequest
	err     error
	i       int
}

func (p *scriptedProvider) Chat(_ context.Context, req provider.ChatRequest) (provider.Message, error) {
	p.got = append(p.got, req)
	if p.err != nil {
		return provider.Message{}, p.err
	}
	if p.i >= len(p.replies) {
		return provider.Message{}, errString("unexpected Chat call")
	}
	msg := p.replies[p.i]
	p.i++
	return msg, nil
}

type errString string

func (e errString) Error() string { return string(e) }
