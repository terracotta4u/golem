package db

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const (
	// FileName is the sqlite database in the Golem directory.
	FileName = "golem.db"

	schemaVersion = 1
)

// schema is the whole application database. conversation_id on memories is
// not a foreign key: a fact outlives the conversation it was learned from.
const schema = `
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

CREATE TABLE IF NOT EXISTS memories (
	id TEXT PRIMARY KEY,
	content TEXT NOT NULL,
	conversation_id TEXT NOT NULL,
	turn_id TEXT NOT NULL,
	created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS memories_by_created ON memories (created_at DESC);

CREATE TABLE IF NOT EXISTS embeddings (
	memory_id TEXT PRIMARY KEY,
	provider TEXT NOT NULL,
	model TEXT NOT NULL,
	dimensions INTEGER NOT NULL,
	vector TEXT NOT NULL,
	FOREIGN KEY (memory_id) REFERENCES memories(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS embeddings_by_model ON embeddings (provider, model, dimensions);
`

// Open creates dir if needed and opens dir/golem.db, applying the schema.
// The caller closes the returned database.
func Open(dir string) (*sql.DB, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create %s: %w", dir, err)
	}

	path := filepath.Join(dir, FileName)
	query := url.Values{}
	query.Set("_foreign_keys", "on")
	query.Set("_busy_timeout", "5000")
	query.Set("_journal_mode", "WAL")
	query.Set("_synchronous", "NORMAL")
	query.Set("_txlock", "immediate")
	dsn := (&url.URL{
		Scheme:   "file",
		Path:     filepath.ToSlash(path),
		RawQuery: query.Encode(),
	}).String()

	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	// Remember runs beside request handlers. A small pool plus WAL lets those
	// overlap; immediate transactions avoid the lock upgrade that SQLITE_BUSY
	// comes from.
	sqlDB.SetMaxOpenConns(4)
	sqlDB.SetMaxIdleConns(4)

	if err := migrate(sqlDB); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return sqlDB, nil
}

func migrate(sqlDB *sql.DB) error {
	var version int
	if err := sqlDB.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if version > schemaVersion {
		return fmt.Errorf("golem.db schema version %d is newer than this build (%d)", version, schemaVersion)
	}
	if version == schemaVersion {
		return nil
	}
	if version != 0 {
		return fmt.Errorf("golem.db schema version %d is not supported", version)
	}
	return applyMigration(sqlDB, schema, fmt.Sprintf("PRAGMA user_version = %d", schemaVersion))
}

// applyMigration runs schemaSQL and versionSQL in one transaction.
func applyMigration(sqlDB *sql.DB, schemaSQL, versionSQL string) error {
	tx, err := sqlDB.Begin()
	if err != nil {
		return fmt.Errorf("begin schema: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(schemaSQL); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	if _, err := tx.Exec(versionSQL); err != nil {
		return fmt.Errorf("set schema version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit schema: %w", err)
	}
	return nil
}
