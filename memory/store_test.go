package memory

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveLoad(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "memories.db"))
	if err != nil {
		t.Fatal(err)
	}

	m := Memory{
		ID:             "mem-1",
		Content:        "User prefers the Go standard library.",
		ConversationID: "conv-1",
		TurnID:         "turn-1",
		CreatedAt:      time.Date(2026, 9, 12, 16, 0, 0, 0, time.UTC),
	}
	if err := st.Save(m); err != nil {
		t.Fatal(err)
	}

	got, err := st.Load("mem-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != m.ID || got.Content != m.Content || got.ConversationID != m.ConversationID || got.TurnID != m.TurnID {
		t.Errorf("got %+v", got)
	}
	if !got.CreatedAt.Equal(m.CreatedAt) {
		t.Errorf("created_at = %v, want %v", got.CreatedAt, m.CreatedAt)
	}
}

func TestLoadMissing(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "memories.db"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.Load("missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Load = %v, want ErrNotFound", err)
	}
}

func TestSaveUpsert(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "memories.db"))
	if err != nil {
		t.Fatal(err)
	}

	m := Memory{
		ID:             "mem-1",
		Content:        "first",
		ConversationID: "conv-1",
		TurnID:         "turn-1",
		CreatedAt:      time.Date(2026, 9, 12, 16, 0, 0, 0, time.UTC),
	}
	if err := st.Save(m); err != nil {
		t.Fatal(err)
	}
	m.Content = "updated"
	if err := st.Save(m); err != nil {
		t.Fatal(err)
	}

	got, err := st.Load("mem-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "updated" {
		t.Errorf("content = %q, want updated", got.Content)
	}
}

func TestListNewestFirst(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "memories.db"))
	if err != nil {
		t.Fatal(err)
	}

	older := Memory{
		ID:             "old",
		Content:        "older memory",
		ConversationID: "conv-1",
		TurnID:         "turn-1",
		CreatedAt:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	newer := Memory{
		ID:             "new",
		Content:        "newer memory",
		ConversationID: "conv-2",
		TurnID:         "turn-2",
		CreatedAt:      time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
	}
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
	if list[0].ID != newer.ID || list[0].Content != newer.Content {
		t.Errorf("first = %+v, want newest", list[0])
	}
	if list[1].ID != older.ID {
		t.Errorf("second = %+v, want oldest", list[1])
	}
}

func TestReopenSameFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memories.db")
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	m := Memory{
		ID:             "mem-1",
		Content:        "User prefers the Go standard library.",
		ConversationID: "conv-1",
		TurnID:         "turn-1",
		CreatedAt:      time.Date(2026, 9, 12, 16, 0, 0, 0, time.UTC),
	}
	if err := st.Save(m); err != nil {
		t.Fatal(err)
	}

	st2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := st2.Load("mem-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != m.Content || got.ConversationID != m.ConversationID || got.TurnID != m.TurnID {
		t.Errorf("got %+v", got)
	}
	list, err := st2.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != m.ID {
		t.Fatalf("list = %+v, want rebuilt row for %s", list, m.ID)
	}
}
