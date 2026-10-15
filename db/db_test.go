package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCreatesGolemDB(t *testing.T) {
	dir := t.TempDir()
	sqlDB := openDB(t, dir)
	defer sqlDB.Close()

	path := filepath.Join(dir, FileName)
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "conversations.db")); !os.IsNotExist(err) {
		t.Fatalf("conversations.db: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "memories.db")); !os.IsNotExist(err) {
		t.Fatalf("memories.db: %v", err)
	}

	got := map[string]bool{}
	rows, err := sqlDB.Query(`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		got[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"conversations", "messages", "tool_calls", "memories", "embeddings"} {
		if !got[name] {
			t.Errorf("missing table %s", name)
		}
	}

	var version int
	if err := sqlDB.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != schemaVersion {
		t.Errorf("user_version = %d, want %d", version, schemaVersion)
	}

	var fk string
	if err := sqlDB.QueryRow(`PRAGMA foreign_keys`).Scan(&fk); err != nil {
		t.Fatal(err)
	}
	if fk != "1" {
		t.Errorf("foreign_keys = %q, want 1", fk)
	}

	var journal string
	if err := sqlDB.QueryRow(`PRAGMA journal_mode`).Scan(&journal); err != nil {
		t.Fatal(err)
	}
	if journal != "wal" {
		t.Errorf("journal_mode = %q, want wal", journal)
	}
}

func TestOpenRejectsNewerSchema(t *testing.T) {
	dir := t.TempDir()
	sqlDB := openDB(t, dir)
	if _, err := sqlDB.Exec(`PRAGMA user_version = 99`); err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}

	_, err := Open(dir)
	if err == nil {
		t.Fatal("Open succeeded on a newer schema")
	}
}

func TestForeignKeysCascade(t *testing.T) {
	sqlDB := openDB(t, t.TempDir())
	defer sqlDB.Close()

	if _, err := sqlDB.Exec(`
		INSERT INTO conversations (id, channel, title, updated_at)
		VALUES ('c1', 'cli', 'hello', '2026-01-01T00:00:00Z')
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.Exec(`
		INSERT INTO messages (conversation_id, seq, role, content)
		VALUES ('c1', 0, 'user', 'hi')
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.Exec(`
		INSERT INTO tool_calls (conversation_id, message_seq, call_seq, id, name)
		VALUES ('c1', 0, 0, 'call_1', 'read')
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.Exec(`DELETE FROM conversations WHERE id = 'c1'`); err != nil {
		t.Fatal(err)
	}
	if n := count(t, sqlDB, `SELECT COUNT(*) FROM messages`); n != 0 {
		t.Errorf("messages = %d, want 0", n)
	}
	if n := count(t, sqlDB, `SELECT COUNT(*) FROM tool_calls`); n != 0 {
		t.Errorf("tool_calls = %d, want 0", n)
	}

	if _, err := sqlDB.Exec(`
		INSERT INTO messages (conversation_id, seq, role, content)
		VALUES ('missing', 0, 'user', 'hi')
	`); err == nil {
		t.Fatal("message insert without a conversation succeeded")
	}

	if _, err := sqlDB.Exec(`
		INSERT INTO memories (id, content, conversation_id, turn_id, created_at)
		VALUES ('m1', 'User prefers tea.', 'c1', 't1', '2026-01-01T00:00:00Z')
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.Exec(`
		INSERT INTO embeddings (memory_id, provider, model, dimensions, vector)
		VALUES ('m1', 'openrouter', 'openai/text-embedding-3-small', 1, '[1]')
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.Exec(`
		INSERT INTO embeddings (memory_id, provider, model, dimensions, vector)
		VALUES ('missing', 'openrouter', 'openai/text-embedding-3-small', 1, '[1]')
	`); err == nil {
		t.Fatal("embedding insert without a memory succeeded")
	}
	if _, err := sqlDB.Exec(`DELETE FROM memories WHERE id = 'm1'`); err != nil {
		t.Fatal(err)
	}
	if n := count(t, sqlDB, `SELECT COUNT(*) FROM embeddings`); n != 0 {
		t.Errorf("embeddings = %d, want 0", n)
	}
}

func TestReopenKeepsRows(t *testing.T) {
	dir := t.TempDir()
	sqlDB := openDB(t, dir)
	if _, err := sqlDB.Exec(`
		INSERT INTO memories (id, content, conversation_id, turn_id, created_at)
		VALUES ('m1', 'User prefers tea.', 'c1', 't1', '2026-01-01T00:00:00Z')
	`); err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}

	reopened := openDB(t, dir)
	defer reopened.Close()
	var content string
	if err := reopened.QueryRow(`SELECT content FROM memories WHERE id = 'm1'`).Scan(&content); err != nil {
		t.Fatal(err)
	}
	if content != "User prefers tea." {
		t.Errorf("content = %q", content)
	}
}

func openDB(t *testing.T, dir string) *sql.DB {
	t.Helper()
	sqlDB, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return sqlDB
}

func count(t *testing.T, sqlDB *sql.DB, query string) int {
	t.Helper()
	var n int
	if err := sqlDB.QueryRow(query).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
