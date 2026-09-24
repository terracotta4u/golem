package conversation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/terracotta4u/golem/provider"
)

func TestTitleFrom(t *testing.T) {
	if got := TitleFrom("  hello\nworld  "); got != "hello world" {
		t.Errorf("TitleFrom(multiline) = %q, want hello world", got)
	}
	if got := TitleFrom(`"Dinner plans"`); got != "Dinner plans" {
		t.Errorf("TitleFrom(quoted) = %q, want Dinner plans", got)
	}
	if got := TitleFrom("  "); got != "" {
		t.Errorf("TitleFrom(blank) = %q, want empty", got)
	}
	long := strings.Repeat("a", 90)
	if got := TitleFrom(long); got != strings.Repeat("a", 80) {
		t.Errorf("TitleFrom(long) len = %d, want 80", len(got))
	}
}

func TestSetTitleFromSkipsWhenSet(t *testing.T) {
	c := Conversation{Title: "keep"}
	c.SetTitleFrom("hello")
	if c.Title != "keep" {
		t.Errorf("title = %q, want keep", c.Title)
	}
}

func TestSaveLoad(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	c := New("cli")
	c.Title = "hello"
	c.UpdatedAt = time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	c.Messages = []provider.Message{{Role: "user", Content: "hi"}}

	if err := st.Save(c); err != nil {
		t.Fatal(err)
	}

	got, err := st.Load(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != c.ID || got.Channel != "cli" || got.Title != "hello" {
		t.Errorf("got %+v", got)
	}
	if len(got.Messages) != 1 || got.Messages[0].Content != "hi" {
		t.Errorf("messages = %+v", got.Messages)
	}
	if !got.UpdatedAt.Equal(c.UpdatedAt) {
		t.Errorf("updated_at = %v, want %v", got.UpdatedAt, c.UpdatedAt)
	}
}

func TestListNewestFirstWithoutMessages(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	older := New("cli")
	older.Title = "old"
	older.UpdatedAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	older.Messages = []provider.Message{{Role: "user", Content: "secret"}}

	newer := New("telegram")
	newer.Title = "new"
	newer.UpdatedAt = time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	newer.Messages = []provider.Message{{Role: "user", Content: "also secret"}}

	if err := st.Save(older); err != nil {
		t.Fatal(err)
	}
	if err := st.Save(newer); err != nil {
		t.Fatal(err)
	}

	list, err := st.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("list len = %d, want 2", len(list))
	}
	if list[0].ID != newer.ID || list[0].Title != "new" || list[0].Channel != "telegram" {
		t.Errorf("first = %+v, want newest", list[0])
	}
	if list[1].ID != older.ID {
		t.Errorf("second = %+v, want oldest", list[1])
	}
	if list[0].Messages != nil || list[1].Messages != nil {
		t.Errorf("list should omit messages: %+v", list)
	}
}

func TestClose(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Load("missing"); err == nil {
		t.Fatal("Load after Close succeeded")
	}
}

func TestLoadOrCreateMissingReturnsUnsaved(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	c, err := st.LoadOrCreate("thread-1", "slack")
	if err != nil {
		t.Fatal(err)
	}
	if c.ID != "thread-1" || c.Channel != "slack" {
		t.Errorf("got %+v", c)
	}
	if _, err := st.Load("thread-1"); err != ErrNotFound {
		t.Errorf("Load = %v, want ErrNotFound", err)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	c := New("cli")
	c.Title = "tools"
	c.UpdatedAt = time.Date(2026, 8, 22, 18, 0, 0, 123, time.UTC)
	c.Messages = []provider.Message{
		{
			Role: "assistant",
			ToolCalls: []provider.ToolCall{{
				ID:   "call_1",
				Type: "function",
				Function: provider.FunctionCall{
					Name:      "read",
					Arguments: `{"path":"a.go"}`,
				},
			}},
		},
		{Role: "tool", ToolCallID: "call_1", Content: "package main"},
	}
	if err := st.Save(c); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, "conversations.db")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "conversations")); !os.IsNotExist(err) {
		t.Fatalf("conversations directory present: %v", err)
	}

	st2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := st2.Load(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "tools" || got.Channel != "cli" || !got.UpdatedAt.Equal(c.UpdatedAt) {
		t.Fatalf("got %+v", got)
	}
	if len(got.Messages) != 2 {
		t.Fatalf("messages = %+v", got.Messages)
	}
	call := got.Messages[0].ToolCalls
	if len(call) != 1 || call[0].ID != "call_1" || call[0].Type != "function" || call[0].Function.Name != "read" || call[0].Function.Arguments != `{"path":"a.go"}` {
		t.Fatalf("tool call = %+v", call)
	}
	if got.Messages[1].Role != "tool" || got.Messages[1].ToolCallID != "call_1" || got.Messages[1].Content != "package main" {
		t.Fatalf("tool result = %+v", got.Messages[1])
	}

	c.Title = "renamed"
	c.Messages = []provider.Message{{Role: "user", Content: "next"}}
	if err := st2.Save(c); err != nil {
		t.Fatal(err)
	}
	got, err = st2.Load(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "renamed" || len(got.Messages) != 1 || got.Messages[0].Content != "next" || got.Messages[0].ToolCalls != nil {
		t.Fatalf("after replace = %+v", got)
	}
}
