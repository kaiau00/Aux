package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

func openTestDB(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	return db
}

// readPragmaOnEveryConnection forces the pool to hold n simultaneous
// connections and reads a pragma on each, so the result reflects connections
// the pool opened lazily rather than only the first one.
func readPragmaOnEveryConnection(t *testing.T, db *sql.DB, pragma string, n int) []string {
	t.Helper()

	ctx := context.Background()
	conns := make([]*sql.Conn, 0, n)
	for i := 0; i < n; i++ {
		c, err := db.Conn(ctx)
		if err != nil {
			t.Fatalf("Conn %d: %v", i, err)
		}
		conns = append(conns, c)
	}
	defer func() {
		for _, c := range conns {
			c.Close()
		}
	}()

	values := make([]string, n)
	var wg sync.WaitGroup
	for i, c := range conns {
		wg.Add(1)
		go func(i int, c *sql.Conn) {
			defer wg.Done()
			if err := c.QueryRowContext(ctx, "PRAGMA "+pragma).Scan(&values[i]); err != nil {
				values[i] = "ERR:" + err.Error()
			}
		}(i, c)
	}
	wg.Wait()
	return values
}

func assertAllEqual(t *testing.T, pragma string, got []string, want string) {
	t.Helper()
	for i, v := range got {
		if v != want {
			t.Errorf("connection %d has %s = %q, want %q (all connections must agree)", i, pragma, v, want)
		}
	}
}

// The reason pragmas moved into the DSN: pragmas are per-connection state, and
// an Exec against a pooled *sql.DB reaches only whichever connection served it.
// Every connection the pool opens later must still carry them.
func TestDSNPragmasApplyToEveryConnection(t *testing.T) {
	db := openTestDB(t, DSN(filepath.Join(t.TempDir(), "pragma.db")))
	tunePool(db)

	const conns = 6
	for _, tc := range []struct{ pragma, want string }{
		{"foreign_keys", "1"},
		{"journal_mode", "wal"},
		{"synchronous", "1"}, // NORMAL
		{"busy_timeout", "5000"},
	} {
		assertAllEqual(t, tc.pragma, readPragmaOnEveryConnection(t, db, tc.pragma, conns), tc.want)
	}
}

// Guards the specific trap in this driver: setting any _pragma in the DSN
// disables its automatic one-minute busy timeout. Dropping busy_timeout from
// the DSN would silently leave it at zero, so a blocked writer would fail
// immediately instead of waiting.
func TestBusyTimeoutIsSetExplicitly(t *testing.T) {
	db := openTestDB(t, DSN(filepath.Join(t.TempDir(), "busy.db")))

	var timeout int
	if err := db.QueryRow("PRAGMA busy_timeout").Scan(&timeout); err != nil {
		t.Fatalf("read busy_timeout: %v", err)
	}
	if timeout != busyTimeoutMS {
		t.Fatalf("busy_timeout is %d, want %d; adding a _pragma to the DSN disables the driver default, so this must be explicit", timeout, busyTimeoutMS)
	}
	if timeout == 0 {
		t.Fatal("a zero busy timeout makes concurrent writers fail instead of wait")
	}
}

// SQLite accepts pragmas it does not understand and values it cannot parse,
// doing nothing and reporting success. Only a syntax error fails the open.
//
// This is the whole reason verifyPragmas exists, so the behaviour is pinned
// here: if a future SQLite or driver starts rejecting these, verifyPragmas can
// be reconsidered, and until then it must not be removed as redundant.
func TestSQLiteSilentlyAcceptsMeaninglessPragmas(t *testing.T) {
	open := func(pragma string) error {
		dsn := "file:" + filepath.Join(t.TempDir(), "p.db") + "?_pragma=" + pragma
		db, err := sql.Open("sqlite3", dsn)
		if err != nil {
			return err
		}
		defer db.Close()
		return db.Ping()
	}

	for _, pragma := range []string{
		"journal_mode(not_a_mode)",
		"totally_made_up_pragma(1)",
		"busy_timeout(abc)",
	} {
		if err := open(pragma); err != nil {
			t.Errorf("expected %q to be silently ignored (documented SQLite behaviour), got %v", pragma, err)
		}
	}

	if err := open("busy_timeout(((("); err == nil {
		t.Error("a syntactically invalid pragma should still fail the connection")
	}
}

// The check that catches what SQLite will not: a durability setting that did
// not take effect must stop startup rather than degrade it silently.
func TestVerifyPragmasRejectsAWeakerDatabase(t *testing.T) {
	// A database opened with no pragmas at all: rollback journal, not WAL.
	plain := openTestDB(t, filepath.Join(t.TempDir(), "plain.db"))
	if err := verifyPragmas(plain); err == nil {
		t.Fatal("a database without WAL must be rejected, not accepted quietly")
	} else if !strings.Contains(err.Error(), "journal_mode") {
		t.Fatalf("the error should name what is wrong, got %v", err)
	}

	// The real DSN must pass.
	configured := openTestDB(t, DSN(filepath.Join(t.TempDir(), "ok.db")))
	if err := verifyPragmas(configured); err != nil {
		t.Fatalf("the configured DSN must satisfy its own check, got %v", err)
	}
}

// Parallel read-only tool execution made concurrent database writers routine.
// The idle pool has to be large enough to reuse those connections; at the
// database/sql default of 2, a burst repeatedly opens and closes connections.
func TestPoolIsBoundedAndReusesConnections(t *testing.T) {
	db := openTestDB(t, DSN(filepath.Join(t.TempDir(), "pool.db")))
	tunePool(db)

	if _, err := db.Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	// A burst of concurrent writers, as parallel tool execution produces.
	var wg sync.WaitGroup
	errs := make([]error, 32)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = db.Exec(`INSERT INTO t (v) VALUES (?)`, "row")
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent write %d failed: %v", i, err)
		}
	}

	var count int
	if err := db.QueryRow(`SELECT count(*) FROM t`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 32 {
		t.Fatalf("expected all 32 concurrent writes to land, got %d", count)
	}

	stats := db.Stats()
	if stats.OpenConnections > maxOpenConns {
		t.Fatalf("pool opened %d connections, above the %d bound", stats.OpenConnections, maxOpenConns)
	}
	// With idle capacity matched to the open bound, a burst should never close a
	// connection just because the idle pool was full. Measured at the
	// database/sql default of 2 idle connections, the same burst closes several.
	if stats.MaxIdleClosed != 0 {
		t.Errorf("pool closed %d connections for lack of idle capacity; they should be reused across a burst", stats.MaxIdleClosed)
	}
}
