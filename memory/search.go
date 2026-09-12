package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"
)

func (idx *Index) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	vecs, err := idx.embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if len(vecs) == 0 || len(vecs[0]) == 0 {
		return nil, fmt.Errorf("embed query: empty vector")
	}
	q := vecs[0]

	rows, err := idx.store.db.Query(`
		SELECT m.id, m.content, m.conversation_id, m.turn_id, m.created_at, e.vector
		FROM embeddings e
		JOIN memories m ON m.id = e.memory_id
		WHERE e.provider = ? AND e.model = ? AND e.dimensions = ?
	`, idx.provider, idx.model, len(q))
	if err != nil {
		return nil, fmt.Errorf("search embeddings: %w", err)
	}
	defer rows.Close()

	var out []Result
	for rows.Next() {
		var m Memory
		var created, raw string
		if err := rows.Scan(&m.ID, &m.Content, &m.ConversationID, &m.TurnID, &created, &raw); err != nil {
			return nil, err
		}
		m.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
		if err != nil {
			return nil, fmt.Errorf("parse created_at for %s: %w", m.ID, err)
		}
		var vec []float32
		if err := json.Unmarshal([]byte(raw), &vec); err != nil {
			return nil, fmt.Errorf("parse vector for %s: %w", m.ID, err)
		}
		out = append(out, Result{Memory: m, Score: cosine(q, vec)})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if limit < 0 {
		limit = 0
	}
	if limit < len(out) {
		out = out[:limit]
	}
	return out, nil
}

func cosine(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(na) * math.Sqrt(nb)))
}
