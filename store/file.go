package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type FileStore struct {
	dir string
	db  *sql.DB
}

func NewFileStore(golemDir string) (*FileStore, error) {
	dir := filepath.Join(golemDir, "conversations")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create %s: %w", dir, err)
	}

	db, err := sql.Open("sqlite", filepath.Join(golemDir, "conversations.db"))
	if err != nil {
		return nil, fmt.Errorf("open index: %w", err)
	}
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS conversations (
			id TEXT PRIMARY KEY,
			channel TEXT NOT NULL,
			title TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL
		);
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("create index: %w", err)
	}

	s := &FileStore{dir: dir, db: db}
	if err := s.rebuild(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *FileStore) rebuild() error {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			return err
		}
		var c Conversation
		if err := json.Unmarshal(data, &c); err != nil {
			continue
		}
		if err := s.upsert(c); err != nil {
			return err
		}
	}
	return nil
}

func (s *FileStore) Load(id string) (Conversation, error) {
	data, err := os.ReadFile(s.path(id))
	if os.IsNotExist(err) {
		return Conversation{}, ErrNotFound
	}
	if err != nil {
		return Conversation{}, err
	}
	var c Conversation
	if err := json.Unmarshal(data, &c); err != nil {
		return Conversation{}, fmt.Errorf("parse conversation %s: %w", id, err)
	}
	return c, nil
}

func (s *FileStore) Save(c Conversation) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("encode conversation: %w", err)
	}
	data = append(data, '\n')

	path := s.path(c.ID)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write conversation: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("write conversation: %w", err)
	}

	if err := s.upsert(c); err != nil {
		return err
	}
	return nil
}

func (s *FileStore) List() ([]Conversation, error) {
	rows, err := s.db.Query(`
		SELECT id, channel, title, updated_at
		FROM conversations
		ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Conversation
	for rows.Next() {
		var c Conversation
		var updated string
		if err := rows.Scan(&c.ID, &c.Channel, &c.Title, &updated); err != nil {
			return nil, err
		}
		c.UpdatedAt, err = time.Parse(time.RFC3339Nano, updated)
		if err != nil {
			return nil, fmt.Errorf("parse updated_at for %s: %w", c.ID, err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *FileStore) upsert(c Conversation) error {
	_, err := s.db.Exec(`
		INSERT INTO conversations (id, channel, title, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			channel = excluded.channel,
			title = excluded.title,
			updated_at = excluded.updated_at
	`, c.ID, c.Channel, c.Title, c.UpdatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("index conversation: %w", err)
	}
	return nil
}

func (s *FileStore) path(id string) string {
	sum := sha256.Sum256([]byte(id))
	return filepath.Join(s.dir, hex.EncodeToString(sum[:])+".json")
}
