package db

import (
	"database/sql"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/pressly/goose/v3"
)

// Migrations run on every Connect and releases ship across schema changes, so
// every existing user walks the upgrade path and, until these tests, nothing
// exercised it. Fresh databases were covered by every other test in the tree;
// populated ones by none, which is the only case a user ever has.

func gooseReady(t *testing.T) {
	t.Helper()
	goose.SetBaseFS(FS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("set goose dialect: %v", err)
	}
	goose.SetLogger(goose.NopLogger())
}

// openAt returns a database migrated to exactly version, opened the way
// production opens it so the real pragma set applies.
func openAt(t *testing.T, version int64) *sql.DB {
	t.Helper()
	gooseReady(t)
	conn, err := sql.Open("sqlite3", DSN(filepath.Join(t.TempDir(), "aux.db")))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := goose.UpTo(conn, "migrations", version); err != nil {
		t.Fatalf("migrate up to %d: %v", version, err)
	}
	return conn
}

func allVersions(t *testing.T) []int64 {
	t.Helper()
	gooseReady(t)
	ms, err := goose.CollectMigrations("migrations", 0, math.MaxInt64)
	if err != nil {
		t.Fatalf("collect migrations: %v", err)
	}
	versions := make([]int64, 0, len(ms))
	for _, m := range ms {
		versions = append(versions, m.Version)
	}
	if len(versions) < 2 {
		t.Fatalf("expected several migrations, found %d", len(versions))
	}
	return versions
}

// seed writes rows into the three tables that have existed since the first
// migration, so an identical seed works from any starting version.
func seed(t *testing.T, conn *sql.DB) {
	t.Helper()
	stmts := []string{
		`INSERT INTO sessions (id, title, message_count, prompt_tokens, completion_tokens, cost, updated_at, created_at)
		 VALUES ('sess-upgrade', 'a session that predates the upgrade', 0, 10, 20, 0.5, 1000, 1000)`,
		`INSERT INTO messages (id, session_id, role, parts, model, created_at, updated_at)
		 VALUES ('msg-upgrade', 'sess-upgrade', 'user', '[{"type":"text","text":"survive the upgrade"}]', 'test-model', 1000, 1000)`,
		`INSERT INTO files (id, session_id, path, content, version, created_at, updated_at)
		 VALUES ('file-upgrade', 'sess-upgrade', 'main.go', 'package main', 'v1', 1000, 1000)`,
	}
	for _, s := range stmts {
		if _, err := conn.Exec(s); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
}

// assertSeedIntact checks the seeded rows survived, including the column added
// by a later migration and the trigger-maintained counter -- an upgrade that
// silently drops rows or resets a count is the failure worth catching.
func assertSeedIntact(t *testing.T, conn *sql.DB) {
	t.Helper()
	var title string
	var count int
	if err := conn.QueryRow(`SELECT title, message_count FROM sessions WHERE id = 'sess-upgrade'`).Scan(&title, &count); err != nil {
		t.Fatalf("session did not survive the upgrade: %v", err)
	}
	if title != "a session that predates the upgrade" {
		t.Fatalf("session title changed across the upgrade: %q", title)
	}
	if count != 1 {
		t.Fatalf("message_count is %d, want 1: the insert trigger did not survive", count)
	}

	var parts string
	if err := conn.QueryRow(`SELECT parts FROM messages WHERE id = 'msg-upgrade'`).Scan(&parts); err != nil {
		t.Fatalf("message did not survive the upgrade: %v", err)
	}
	if !strings.Contains(parts, "survive the upgrade") {
		t.Fatalf("message content changed across the upgrade: %q", parts)
	}

	var content string
	if err := conn.QueryRow(`SELECT content FROM files WHERE id = 'file-upgrade'`).Scan(&content); err != nil {
		t.Fatalf("file did not survive the upgrade: %v", err)
	}
	if content != "package main" {
		t.Fatalf("file content changed across the upgrade: %q", content)
	}
}

// A user upgrading arrives from whatever version they last ran, not only from
// the newest one, so every version has to reach current with its data intact.
func TestUpgradeFromEveryVersionPreservesData(t *testing.T) {
	versions := allVersions(t)
	latest := versions[len(versions)-1]

	for _, from := range versions {
		t.Run(fmt.Sprint(from), func(t *testing.T) {
			conn := openAt(t, from)
			seed(t, conn)

			if err := goose.Up(conn, "migrations"); err != nil {
				t.Fatalf("upgrade from %d to current failed: %v", from, err)
			}

			got, err := goose.GetDBVersion(conn)
			if err != nil {
				t.Fatalf("read version: %v", err)
			}
			if got != latest {
				t.Fatalf("after upgrade the database is at %d, want %d", got, latest)
			}
			assertSeedIntact(t, conn)
		})
	}
}

// Upgrading twice must be a no-op rather than an error: Connect runs goose.Up
// on every single start, so the second run is the common case, not an edge one.
func TestUpgradeIsIdempotent(t *testing.T) {
	versions := allVersions(t)
	conn := openAt(t, versions[0])
	seed(t, conn)

	for i := range 3 {
		if err := goose.Up(conn, "migrations"); err != nil {
			t.Fatalf("goose.Up run %d failed: %v", i+1, err)
		}
	}
	assertSeedIntact(t, conn)
}

// The dangerous direction. goose.Up is perfectly happy with a database newer
// than the binary -- there is nothing left to apply, so it succeeds -- and the
// old binary then runs against a schema it does not know. The first symptom
// would be some unrelated query failing on a column, long after the cause.
func TestDatabaseFromANewerBuildIsRefused(t *testing.T) {
	versions := allVersions(t)
	conn := openAt(t, versions[len(versions)-1])

	future := versions[len(versions)-1] + 1
	if _, err := conn.Exec(
		`INSERT INTO goose_db_version (version_id, is_applied, tstamp) VALUES (?, 1, CURRENT_TIMESTAMP)`,
		future,
	); err != nil {
		t.Fatalf("stamp a future version: %v", err)
	}

	err := ensureNotNewer(conn, "/tmp/aux.db")
	if err == nil {
		t.Fatal("a database written by a newer build was accepted; the schema mismatch would surface later as an unrelated query error")
	}
	// The message is the entire point of the check: it has to name the cause
	// and the fix, because the user cannot see a version number themselves.
	for _, want := range []string{"newer version", "Upgrade aux"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error does not tell the user what happened or what to do (missing %q): %v", want, err)
		}
	}
}

func TestCurrentAndFreshDatabasesAreAccepted(t *testing.T) {
	versions := allVersions(t)

	t.Run("current", func(t *testing.T) {
		conn := openAt(t, versions[len(versions)-1])
		if err := ensureNotNewer(conn, "/tmp/aux.db"); err != nil {
			t.Fatalf("a fully migrated database was refused: %v", err)
		}
	})

	t.Run("older", func(t *testing.T) {
		conn := openAt(t, versions[0])
		if err := ensureNotNewer(conn, "/tmp/aux.db"); err != nil {
			t.Fatalf("an older database was refused; upgrading it is the normal path: %v", err)
		}
	})
}
