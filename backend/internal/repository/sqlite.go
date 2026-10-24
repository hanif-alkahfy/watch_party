package repository

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

type SQLite struct {
	db *sql.DB
}

func OpenSQLite(path string) (*SQLite, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}

	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	// Graceful migration for existing SQLite DBs when columns are added
	_, _ = db.Exec(`ALTER TABLE session_invites ADD COLUMN expires_at DATETIME`)
	_, _ = db.Exec(`ALTER TABLE sessions ADD COLUMN duration_seconds REAL NOT NULL DEFAULT 0`)
	// Legacy single-use rows have no expiry set: treat them as expired.
	_, _ = db.Exec(`UPDATE session_invites SET expires_at = created_at WHERE expires_at IS NULL`)

	return &SQLite{db: db}, nil
}

func (s *SQLite) Close() error { return s.db.Close() }

const timeLayout = time.RFC3339

func nullString(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func scanNullString(v sql.NullString) string {
	if v.Valid {
		return v.String
	}
	return ""
}

func toTime(s string) time.Time {
	t, err := time.Parse(timeLayout, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func now() string { return time.Now().UTC().Format(timeLayout) }
