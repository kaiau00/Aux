package db

import (
	"database/sql"
	"fmt"
	"math"
	"os"
	"path/filepath"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"

	"github.com/aux-ai/aux-cli/internal/config"
	"github.com/aux-ai/aux-cli/internal/logging"

	"github.com/pressly/goose/v3"
)

func Connect() (*sql.DB, error) {
	dataDir := config.Get().Data.Directory
	if dataDir == "" {
		return nil, fmt.Errorf("data.dir is not set")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}
	dbPath := filepath.Join(dataDir, "aux.db")
	db, err := sql.Open("sqlite3", DSN(dbPath))
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Verify connection. Pragmas are applied at connect time, so a bad one
	// surfaces here rather than being logged and ignored.
	if err = db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	tunePool(db)

	if err := verifyPragmas(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("database is not configured as expected: %w", err)
	}

	goose.SetBaseFS(FS)

	if err := goose.SetDialect("sqlite3"); err != nil {
		logging.Error("Failed to set dialect", "error", err)
		return nil, fmt.Errorf("failed to set dialect: %w", err)
	}

	if err := ensureNotNewer(db, dbPath); err != nil {
		db.Close()
		return nil, err
	}

	if err := goose.Up(db, "migrations"); err != nil {
		failure := migrationFailure(db, dbPath, err)
		logging.Error("Failed to apply migrations", "error", err)
		db.Close()
		return nil, failure
	}
	return db, nil
}

// latestEmbeddedVersion is the highest migration version compiled into this
// binary. Requires goose.SetBaseFS to have been called.
func latestEmbeddedVersion() (int64, error) {
	ms, err := goose.CollectMigrations("migrations", 0, math.MaxInt64)
	if err != nil {
		return 0, err
	}
	last, err := ms.Last()
	if err != nil {
		return 0, err
	}
	return last.Version, nil
}

// ensureNotNewer refuses a database stamped by a build newer than this one.
// goose.Up accepts one happily -- there is nothing left to apply, so it
// succeeds -- and the older binary then runs against a schema it does not know,
// where the first symptom is some unrelated query failing on a missing column,
// far from the cause. Going backwards is deliberately not offered: the Down
// migrations exist but have never been run against real data.
func ensureNotNewer(db *sql.DB, dbPath string) error {
	latest, err := latestEmbeddedVersion()
	if err != nil {
		return fmt.Errorf("failed to read this build's migrations: %w", err)
	}
	current, err := goose.GetDBVersion(db)
	if err != nil {
		return fmt.Errorf("failed to read the schema version of %s: %w", dbPath, err)
	}
	if current > latest {
		return fmt.Errorf(
			"the database at %s is at schema version %d, but this build of aux only understands up to %d. "+
				"It was written by a newer version of aux -- Upgrade aux to open it",
			dbPath, current, latest)
	}
	return nil
}

// migrationFailure turns a goose error into something the user can act on.
// Every migration in this tree runs inside a transaction, so a failure leaves
// the database at the last version that applied cleanly rather than halfway
// through one. Saying that is the difference between "my database is corrupt"
// and "it stopped at a known point and retrying is safe".
func migrationFailure(db *sql.DB, dbPath string, cause error) error {
	stoppedAt, verr := goose.GetDBVersion(db)
	if verr != nil {
		return fmt.Errorf(
			"failed to upgrade the database at %s: %w. "+
				"Its schema version could not be read either (%v), so the file may be damaged. "+
				"Move it aside to start with a new one -- sessions and history live there and would be lost",
			dbPath, cause, verr)
	}
	return fmt.Errorf(
		"failed to upgrade the database at %s: %w. "+
			"Each migration runs in a transaction, so the database is intact at schema version %d and the "+
			"failed step was rolled back. Re-running aux retries it. If it keeps failing, report the error "+
			"above, or move the file aside to start with a new one -- sessions and history would be lost",
		dbPath, cause, stoppedAt)
}
