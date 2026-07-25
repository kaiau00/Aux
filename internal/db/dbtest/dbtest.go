// Package dbtest provides an isolated, fully-migrated SQLite database for tests
// across the domain packages. It applies the same embedded goose migrations used
// in production so schema tests exercise the real DDL.
package dbtest

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/pressly/goose/v3"

	"github.com/aux-ai/aux-cli/internal/db"
)

// New opens a fresh temp-file SQLite database, applies all migrations, and
// registers cleanup. Each call returns an independent database.
func New(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "aux-test.db")
	conn, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	for _, pragma := range []string{
		"PRAGMA foreign_keys = ON;",
		"PRAGMA journal_mode = WAL;",
		"PRAGMA synchronous = NORMAL;",
	} {
		if _, err := conn.Exec(pragma); err != nil {
			t.Fatalf("pragma %q: %v", pragma, err)
		}
	}

	goose.SetBaseFS(db.FS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("set goose dialect: %v", err)
	}
	goose.SetLogger(goose.NopLogger())
	if err := goose.Up(conn, "migrations"); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	return conn
}
