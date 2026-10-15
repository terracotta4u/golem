package conversation

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/terracotta4u/golem/db"
	"github.com/terracotta4u/golem/provider"
)

type DB struct {
	db    *sql.DB
	owned bool
}

// Open opens golem.db in dir. Close releases that connection.
func Open(dir string) (*DB, error) {
	sqlDB, err := db.Open(dir)
	if err != nil {
		return nil, err
	}
	return &DB{db: sqlDB, owned: true}, nil
}

// Share uses an open golem.db. The caller closes sqlDB.
func Share(sqlDB *sql.DB) *DB {
	return &DB{db: sqlDB}
}

func (s *DB) Close() error {
	if !s.owned || s.db == nil {
		return nil
	}
	return s.db.Close()
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
