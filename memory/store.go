package memory

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/terracotta4u/golem/db"
)

type Store struct {
	db    *sql.DB
	owned bool
}

// Open opens golem.db in dir. Close releases that connection.
func Open(dir string) (*Store, error) {
	sqlDB, err := db.Open(dir)
	if err != nil {
		return nil, err
	}
	return &Store{db: sqlDB, owned: true}, nil
}

// Share uses an open golem.db. The caller closes sqlDB.
func Share(sqlDB *sql.DB) *Store {
	return &Store{db: sqlDB}
}

func (s *Store) Close() error {
	if !s.owned || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Save(m Memory) error {
	_, err := s.db.Exec(`
		INSERT INTO memories (id, content, conversation_id, turn_id, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			content = excluded.content,
			conversation_id = excluded.conversation_id,
			turn_id = excluded.turn_id,
			created_at = excluded.created_at
	`, m.ID, m.Content, m.ConversationID, m.TurnID, m.CreatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("save memory: %w", err)
	}
	return nil
}

func (s *Store) SaveExtracted(contents []string, conversationID, turnID string) ([]Memory, error) {
	var saved []Memory
	now := time.Now().UTC()
	for _, content := range contents {
		content = strings.TrimSpace(content)
		if content == "" {
			continue
		}
		m := Memory{
			ID:             uuid.NewString(),
			Content:        content,
			ConversationID: conversationID,
			TurnID:         turnID,
			CreatedAt:      now,
		}
		if err := s.Save(m); err != nil {
			return saved, err
		}
		saved = append(saved, m)
	}
	return saved, nil
}

func (s *Store) Load(id string) (Memory, error) {
	var m Memory
	var created string
	err := s.db.QueryRow(`
		SELECT id, content, conversation_id, turn_id, created_at
		FROM memories
		WHERE id = ?
	`, id).Scan(&m.ID, &m.Content, &m.ConversationID, &m.TurnID, &created)
	if err == sql.ErrNoRows {
		return Memory{}, ErrNotFound
	}
	if err != nil {
		return Memory{}, fmt.Errorf("load memory %s: %w", id, err)
	}
	m.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return Memory{}, fmt.Errorf("parse created_at for %s: %w", id, err)
	}
	return m, nil
}

func (s *Store) List() ([]Memory, error) {
	rows, err := s.db.Query(`
		SELECT id, content, conversation_id, turn_id, created_at
		FROM memories
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Memory
	for rows.Next() {
		var m Memory
		var created string
		if err := rows.Scan(&m.ID, &m.Content, &m.ConversationID, &m.TurnID, &created); err != nil {
			return nil, err
		}
		m.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
		if err != nil {
			return nil, fmt.Errorf("parse created_at for %s: %w", m.ID, err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
