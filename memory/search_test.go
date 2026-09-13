package memory

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestSearchRanksByCosine(t *testing.T) {
	st, idx := searchFixture(t)
	query := "Should I add a router dependency?"
	stdlib := Memory{
		ID:             "mem-stdlib",
		Content:        "User prefers the Go standard library.",
		ConversationID: "conv-1",
		TurnID:         "turn-1",
		CreatedAt:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	vintage := Memory{
		ID:             "mem-vintage",
		Content:        "User likes vintage computers.",
		ConversationID: "conv-1",
		TurnID:         "turn-2",
		CreatedAt:      time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
	}
	if err := st.Save(stdlib); err != nil {
		t.Fatal(err)
	}
	if err := st.Save(vintage); err != nil {
		t.Fatal(err)
	}
	if err := idx.Index(context.Background(), stdlib); err != nil {
		t.Fatal(err)
	}
	if err := idx.Index(context.Background(), vintage); err != nil {
		t.Fatal(err)
	}

	got, err := idx.Search(context.Background(), query, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("results = %d, want 2 (weak hits included)", len(got))
	}
	if got[0].Memory.ID != stdlib.ID {
		t.Errorf("first = %s, want stdlib", got[0].Memory.ID)
	}
	if !near(got[0].Score, 1, 0.001) {
		t.Errorf("stdlib score = %v, want ~1", got[0].Score)
	}
	if got[1].Memory.ID != vintage.ID {
		t.Errorf("second = %s, want vintage", got[1].Memory.ID)
	}
	if !near(got[1].Score, 0, 0.001) {
		t.Errorf("vintage score = %v, want ~0", got[1].Score)
	}
}

func TestSearchOmitsUnindexedAndWrongDimensions(t *testing.T) {
	st, idx := searchFixture(t)
	query := "Should I add a router dependency?"
	indexed := Memory{
		ID:             "mem-indexed",
		Content:        "User prefers the Go standard library.",
		ConversationID: "conv-1",
		TurnID:         "turn-1",
		CreatedAt:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	unindexed := Memory{
		ID:             "mem-unindexed",
		Content:        "User likes vintage computers.",
		ConversationID: "conv-1",
		TurnID:         "turn-2",
		CreatedAt:      time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
	}
	wrongDim := Memory{
		ID:             "mem-wrong-dim",
		Content:        "User is building Golem.",
		ConversationID: "conv-1",
		TurnID:         "turn-3",
		CreatedAt:      time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
	}
	for _, m := range []Memory{indexed, unindexed, wrongDim} {
		if err := st.Save(m); err != nil {
			t.Fatal(err)
		}
	}
	if err := idx.Index(context.Background(), indexed); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`
		INSERT INTO embeddings (memory_id, provider, model, dimensions, vector)
		VALUES (?, 'openrouter', 'openai/text-embedding-3-small', 3, '[0.1,0.2,0.3]')
	`, wrongDim.ID); err != nil {
		t.Fatal(err)
	}

	got, err := idx.Search(context.Background(), query, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Memory.ID != indexed.ID {
		t.Fatalf("results = %+v, want only indexed", got)
	}
}

func searchFixture(t *testing.T) (*Store, *Index) {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "memories.db"))
	if err != nil {
		t.Fatal(err)
	}
	query := "Should I add a router dependency?"
	emb := &stubEmbedder{byText: map[string][]float32{
		"User prefers the Go standard library.": {1, 0},
		"User likes vintage computers.":         {0, 1},
		"User is building Golem.":               {1, 0},
		query:                                   {1, 0},
	}}
	idx, err := NewIndex(st, emb, "openrouter", "openai/text-embedding-3-small")
	if err != nil {
		t.Fatal(err)
	}
	return st, idx
}

func near(got, want, eps float32) bool {
	d := got - want
	if d < 0 {
		d = -d
	}
	return d <= eps
}
