package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/terracotta4u/golem/provider"
)

func TestSaveLoad(t *testing.T) {
	st, err := NewFileStore(t.TempDir())
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
	st, err := NewFileStore(t.TempDir())
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

func TestOpenMissingReturnsUnsaved(t *testing.T) {
	st, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	c, err := Open(st, "thread-1", "slack")
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

func TestRebuildIndexFromJSON(t *testing.T) {
	dir := t.TempDir()
	st, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	c := New("cli")
	c.Title = "from json"
	c.UpdatedAt = time.Date(2026, 8, 22, 18, 0, 0, 0, time.UTC)
	c.Messages = []provider.Message{{Role: "user", Content: "hi"}}
	if err := st.Save(c); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(filepath.Join(dir, "conversations", "conversations.db")); err != nil {
		t.Fatal(err)
	}

	st2, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	list, err := st2.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != c.ID || list[0].Title != "from json" || list[0].Channel != "cli" {
		t.Fatalf("list = %+v, want rebuilt metadata for %s", list, c.ID)
	}
}
