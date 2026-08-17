package db

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Connection tuning. These are modest numbers for a single-user desktop tool,
// not a server.
const (
	// maxOpenConns bounds how many SQLite connections exist at once. SQLite
	// admits many readers and one writer under WAL, so a small pool is enough;
	// the bound exists so a burst of parallel tool calls cannot open an
	// unbounded number of connections against one file.
	maxOpenConns = 8

	// maxIdleConns must be close to maxOpenConns. database/sql closes any
	// connection returned to a full idle pool, so leaving this at the default
	// of 2 means a burst of parallel work repeatedly opens and closes
	// connections instead of reusing them.
	maxIdleConns = 8

	// busyTimeoutMS is how long a blocked writer waits for the write lock
	// before failing.
	//
	// Setting any _pragma in the DSN disables this driver's automatic
	// one-minute busy timeout, so it must be set explicitly here or it would
	// silently drop to SQLite's default of zero. A minute is far too long for
	// an interactive tool — a write blocked that long is a bug to surface, not
	// to wait out — so this is deliberately shorter.
	busyTimeoutMS = 5000
)

// connectionPragmas are applied to every connection at open time.
//
// They live in the DSN rather than in a db.Exec after opening because most
// pragmas are per-connection state, and an Exec against a pooled *sql.DB
// applies only to whichever connection happens to serve it. Any connection the
// pool opens later would silently miss them.
//
// In practice this driver already defaults foreign_keys on and journal_mode is
// persisted in the database file, so the correctness-critical settings were
// never actually at risk — this makes the guarantee explicit rather than
// incidental, and picks up synchronous and cache_size, which genuinely were
// applied to only one connection.
var connectionPragmas = []string{
	"foreign_keys(on)",
	"journal_mode(wal)",
	// WAL plus NORMAL is the standard durability trade for a local tool: no
	// fsync per commit, and a crash can lose the most recent transactions but
	// cannot corrupt the database.
	"synchronous(normal)",
	"cache_size(-8000)",
	"busy_timeout(" + strconv.Itoa(busyTimeoutMS) + ")",
}

// DSN builds the connection string for a database file, carrying the pragmas
// every connection needs. Exported so tests and tooling open the database the
// same way the application does.
func DSN(path string) string {
	q := url.Values{}
	for _, p := range connectionPragmas {
		q.Add("_pragma", p)
	}
	// A file: URI is required for the driver to parse query parameters at all.
	return "file:" + path + "?" + q.Encode()
}

// tunePool applies the connection-pool bounds.
func tunePool(db *sql.DB) {
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
}

// verifyPragmas checks that the settings the database's durability depends on
// actually took effect.
//
// SQLite silently ignores pragmas it does not recognize and values it cannot
// parse: `PRAGMA totally_made_up(1)` and `PRAGMA busy_timeout(abc)` both
// succeed and do nothing. Only a syntax error surfaces. So a typo in the pragma
// list, or a pragma dropped by a future SQLite build, would leave the database
// running in a weaker mode with nothing said about it — journal_mode falling
// back to rollback, or busy_timeout to zero.
//
// Reading the values back is the only way to know, and it is cheap to do once
// at startup.
func verifyPragmas(db *sql.DB) error {
	var journalMode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		return fmt.Errorf("failed to read journal_mode: %w", err)
	}
	if !strings.EqualFold(journalMode, "wal") {
		return fmt.Errorf("journal_mode is %q, want wal: the database would run without write-ahead logging", journalMode)
	}

	var busyTimeout int
	if err := db.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		return fmt.Errorf("failed to read busy_timeout: %w", err)
	}
	if busyTimeout == 0 {
		return errors.New("busy_timeout is 0: concurrent writers would fail immediately instead of waiting")
	}
	return nil
}
