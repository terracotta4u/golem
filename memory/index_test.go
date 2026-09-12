package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestIndexWritesEmbedding(t *testing.T) {
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

	emb := &stubEmbedder{vecs: [][]float32{{0.25, 0.5, 0.75}}}
	idx, err := NewIndex(st, emb, "openrouter", "openai/text-embedding-3-small")
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.Index(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	if len(emb.got) != 1 || emb.got[0] != m.Content {
		t.Errorf("embedded %v, want %q", emb.got, m.Content)
	}

	provider, model, dims, vec := mustEmbedding(t, st, m.ID)
	if provider != "openrouter" || model != "openai/text-embedding-3-small" {
		t.Errorf("space = %s/%s", provider, model)
	}
	if dims != 3 {
		t.Errorf("dimensions = %d, want 3", dims)
	}
	if len(vec) != 3 || vec[0] != 0.25 || vec[1] != 0.5 || vec[2] != 0.75 {
		t.Errorf("vector = %v", vec)
	}
}

func TestIndexEmbedErrorLeavesMemory(t *testing.T) {
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

	idx, err := NewIndex(st, &stubEmbedder{err: errString("timeout")}, "openrouter", "openai/text-embedding-3-small")
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.Index(context.Background(), m); err == nil {
		t.Fatal("want embed error")
	}

	got, err := st.Load(m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != m.Content {
		t.Errorf("memory = %+v", got)
	}
	if embeddingExists(t, st, m.ID) {
		t.Error("embedding row written despite embed error")
	}
}

func TestRebuildReindexesFromMemories(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "memories.db"))
	if err != nil {
		t.Fatal(err)
	}
	one := Memory{
		ID:             "mem-1",
		Content:        "User prefers the Go standard library.",
		ConversationID: "conv-1",
		TurnID:         "turn-1",
		CreatedAt:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	two := Memory{
		ID:             "mem-2",
		Content:        "User is building Golem in Go.",
		ConversationID: "conv-1",
		TurnID:         "turn-2",
		CreatedAt:      time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
	}
	if err := st.Save(one); err != nil {
		t.Fatal(err)
	}
	if err := st.Save(two); err != nil {
		t.Fatal(err)
	}

	emb := &stubEmbedder{vecs: [][]float32{{0.25, 0.5}}}
	idx, err := NewIndex(st, emb, "openrouter", "openai/text-embedding-3-small")
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.Index(context.Background(), one); err != nil {
		t.Fatal(err)
	}
	if err := idx.Index(context.Background(), two); err != nil {
		t.Fatal(err)
	}

	if _, err := st.db.Exec(`DELETE FROM embeddings`); err != nil {
		t.Fatal(err)
	}
	if embeddingExists(t, st, one.ID) || embeddingExists(t, st, two.ID) {
		t.Fatal("embeddings still present after delete")
	}

	if err := idx.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{one.ID, two.ID} {
		provider, model, dims, vec := mustEmbedding(t, st, id)
		if provider != "openrouter" || model != "openai/text-embedding-3-small" {
			t.Errorf("%s space = %s/%s", id, provider, model)
		}
		if dims != 2 || len(vec) != 2 {
			t.Errorf("%s dims = %d vec = %v", id, dims, vec)
		}
	}
}

func TestRebuildEmpty(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "memories.db"))
	if err != nil {
		t.Fatal(err)
	}
	idx, err := NewIndex(st, &stubEmbedder{vecs: [][]float32{{1}}}, "openrouter", "openai/text-embedding-3-small")
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
}

type stubEmbedder struct {
	vecs [][]float32
	got  []string
	err  error
}

func (s *stubEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	s.got = append([]string(nil), texts...)
	return s.vecs, s.err
}

func mustEmbedding(t *testing.T, st *Store, id string) (provider, model string, dims int, vec []float32) {
	t.Helper()
	var raw string
	err := st.db.QueryRow(`
		SELECT provider, model, dimensions, vector
		FROM embeddings
		WHERE memory_id = ?
	`, id).Scan(&provider, &model, &dims, &raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(raw), &vec); err != nil {
		t.Fatal(err)
	}
	return provider, model, dims, vec
}

func embeddingExists(t *testing.T, st *Store, id string) bool {
	t.Helper()
	var n int
	err := st.db.QueryRow(`SELECT COUNT(*) FROM embeddings WHERE memory_id = ?`, id).Scan(&n)
	if err == sql.ErrNoRows {
		return false
	}
	if err != nil {
		t.Fatal(err)
	}
	return n > 0
}
