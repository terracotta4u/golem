package memory

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/terracotta4u/golem/provider"
)

// Indexer writes embeddings for a memory. *Index implements it.
type Indexer interface {
	Index(ctx context.Context, m Memory) error
}

type Index struct {
	store    *Store
	embedder provider.Embedder
	provider string
	model    string
}

func NewIndex(st *Store, embedder provider.Embedder, providerName, model string) *Index {
	return &Index{
		store:    st,
		embedder: embedder,
		provider: providerName,
		model:    model,
	}
}

// Index embeds a memory and saves it to the database.
func (idx *Index) Index(ctx context.Context, m Memory) error {
	vecs, err := idx.embedder.Embed(ctx, []string{m.Content})
	if err != nil {
		return fmt.Errorf("embed memory %s: %w", m.ID, err)
	}
	if len(vecs) == 0 || len(vecs[0]) == 0 {
		return fmt.Errorf("embed memory %s: empty vector", m.ID)
	}
	raw, err := json.Marshal(vecs[0])
	if err != nil {
		return fmt.Errorf("encode embedding: %w", err)
	}
	_, err = idx.store.db.Exec(`
		INSERT INTO embeddings (memory_id, provider, model, dimensions, vector)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(memory_id) DO UPDATE SET
			provider = excluded.provider,
			model = excluded.model,
			dimensions = excluded.dimensions,
			vector = excluded.vector
	`, m.ID, idx.provider, idx.model, len(vecs[0]), string(raw))
	if err != nil {
		return fmt.Errorf("save embedding: %w", err)
	}
	return nil
}

func (idx *Index) Rebuild(ctx context.Context) error {
	memories, err := idx.store.List()
	if err != nil {
		return err
	}
	for _, m := range memories {
		if err := idx.Index(ctx, m); err != nil {
			return err
		}
	}
	return nil
}
