package conversation

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/terracotta4u/golem/provider"

	_ "modernc.org/sqlite"
)

type DB struct {
	db *sql.DB
}

func Open(golemDir string) (*DB, error) {
	if err := os.MkdirAll(golemDir, 0o700); err != nil {
		return nil, fmt.Errorf("create %s: %w", golemDir, err)
	}

	dsn := (&url.URL{
		Scheme:   "file",
		Path:     filepath.ToSlash(filepath.Join(golemDir, "conversations.db")),
		RawQuery: "_foreign_keys=1&_busy_timeout=5000",
	}).String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open conversations: %w", err)
	}
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS conversations (
			id TEXT PRIMARY KEY,
			channel TEXT NOT NULL,
			title TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS messages (
			conversation_id TEXT NOT NULL,
			seq INTEGER NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL DEFAULT '',
			tool_call_id TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (conversation_id, seq),
			FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
		);
		CREATE TABLE IF NOT EXISTS tool_calls (
			conversation_id TEXT NOT NULL,
			message_seq INTEGER NOT NULL,
			call_seq INTEGER NOT NULL,
			id TEXT NOT NULL,
			type TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL,
			arguments TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (conversation_id, message_seq, call_seq),
			FOREIGN KEY (conversation_id, message_seq)
				REFERENCES messages(conversation_id, seq) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS conversations_by_updated ON conversations (updated_at DESC);
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("create conversations: %w", err)
	}
	return &DB{db: db}, nil
}

func (s *DB) Load(id string) (Conversation, error) {
	var c Conversation
	var updated string
	err := s.db.QueryRow(`
		SELECT id, channel, title, updated_at
		FROM conversations
		WHERE id = ?
	`, id).Scan(&c.ID, &c.Channel, &c.Title, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return Conversation{}, ErrNotFound
	}
	if err != nil {
		return Conversation{}, err
	}
	c.UpdatedAt, err = parseUpdated(c.ID, updated)
	if err != nil {
		return Conversation{}, err
	}
	c.Messages, err = s.loadMessages(id)
	if err != nil {
		return Conversation{}, err
	}
	return c, nil
}

func (s *DB) loadMessages(id string) ([]provider.Message, error) {
	rows, err := s.db.Query(`
		SELECT seq, role, content, tool_call_id
		FROM messages
		WHERE conversation_id = ?
		ORDER BY seq
	`, id)
	if err != nil {
		return nil, err
	}

	bySeq := map[int]int{}
	var msgs []provider.Message
	for rows.Next() {
		var seq int
		var m provider.Message
		if err := rows.Scan(&seq, &m.Role, &m.Content, &m.ToolCallID); err != nil {
			rows.Close()
			return nil, err
		}
		bySeq[seq] = len(msgs)
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	if len(msgs) == 0 {
		return nil, nil
	}

	calls, err := s.db.Query(`
		SELECT message_seq, id, type, name, arguments
		FROM tool_calls
		WHERE conversation_id = ?
		ORDER BY message_seq, call_seq
	`, id)
	if err != nil {
		return nil, err
	}
	defer calls.Close()

	for calls.Next() {
		var seq int
		var call provider.ToolCall
		if err := calls.Scan(&seq, &call.ID, &call.Type, &call.Function.Name, &call.Function.Arguments); err != nil {
			return nil, err
		}
		i, ok := bySeq[seq]
		if !ok {
			return nil, fmt.Errorf("tool call for missing message %d in %s", seq, id)
		}
		msgs[i].ToolCalls = append(msgs[i].ToolCalls, call)
	}
	return msgs, calls.Err()
}

func (s *DB) Save(c Conversation) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("save conversation: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		INSERT INTO conversations (id, channel, title, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			channel = excluded.channel,
			title = excluded.title,
			updated_at = excluded.updated_at
	`, c.ID, c.Channel, c.Title, formatUpdated(c.UpdatedAt)); err != nil {
		return fmt.Errorf("save conversation: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM tool_calls WHERE conversation_id = ?`, c.ID); err != nil {
		return fmt.Errorf("save conversation: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM messages WHERE conversation_id = ?`, c.ID); err != nil {
		return fmt.Errorf("save conversation: %w", err)
	}
	for seq, m := range c.Messages {
		if _, err := tx.Exec(`
			INSERT INTO messages (conversation_id, seq, role, content, tool_call_id)
			VALUES (?, ?, ?, ?, ?)
		`, c.ID, seq, m.Role, m.Content, m.ToolCallID); err != nil {
			return fmt.Errorf("save message %d: %w", seq, err)
		}
		for callSeq, call := range m.ToolCalls {
			if _, err := tx.Exec(`
				INSERT INTO tool_calls (
					conversation_id, message_seq, call_seq, id, type, name, arguments
				) VALUES (?, ?, ?, ?, ?, ?, ?)
			`, c.ID, seq, callSeq, call.ID, call.Type, call.Function.Name, call.Function.Arguments); err != nil {
				return fmt.Errorf("save tool call %d.%d: %w", seq, callSeq, err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("save conversation: %w", err)
	}
	return nil
}

func (s *DB) List() ([]Conversation, error) {
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
		c.UpdatedAt, err = parseUpdated(c.ID, updated)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func formatUpdated(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000000000Z")
}

func parseUpdated(id, updated string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, updated)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse updated_at for %s: %w", id, err)
	}
	return t, nil
}
