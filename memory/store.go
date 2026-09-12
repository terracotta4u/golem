package memory

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open memories: %w", err)
	}
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS memories (
			id TEXT PRIMARY KEY,
			content TEXT NOT NULL,
			conversation_id TEXT NOT NULL,
			turn_id TEXT NOT NULL,
			created_at TEXT NOT NULL
		);
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("create memories: %w", err)
	}
	return &Store{db: db}, nil
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
