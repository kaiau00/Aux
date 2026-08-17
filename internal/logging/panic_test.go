package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func panicLogs(t *testing.T, dir string) []os.DirEntry {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var found []os.DirEntry
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "aux-panic-") {
			found = append(found, e)
		}
	}
	return found
}

// A crash log belongs with the rest of Aux's state. Written to the working
// directory it lands in whatever repository the user happened to run from,
// where it is both untracked clutter they may commit by accident and the last
// place they would think to look for it.
func TestRecoverPanicWritesToPanicDir(t *testing.T) {
	workdir := t.TempDir()
	t.Chdir(workdir)

	state := filepath.Join(t.TempDir(), "state")
	old := PanicDir
	PanicDir = state
	t.Cleanup(func() { PanicDir = old })

	func() {
		defer RecoverPanic("unit", nil)
		panic("boom")
	}()

	logs := panicLogs(t, state)
	if len(logs) != 1 {
		t.Fatalf("expected one crash log in the panic directory, found %d", len(logs))
	}
	if stray := panicLogs(t, workdir); len(stray) != 0 {
		t.Fatalf("crash log written to the working directory: %v", stray[0].Name())
	}

	// Same reasoning as the debug log: a panic value or stack can carry
	// anything the agent was holding when it died.
	info, err := logs[0].Info()
	if err != nil {
		t.Fatalf("stat crash log: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("crash log mode is %v, want 0600", perm)
	}
}

// Until config has run -- and in tests -- there is no configured directory, and
// a crash must still be recorded rather than silently lost.
func TestRecoverPanicFallsBackToWorkingDirectory(t *testing.T) {
	workdir := t.TempDir()
	t.Chdir(workdir)

	old := PanicDir
	PanicDir = ""
	t.Cleanup(func() { PanicDir = old })

	func() {
		defer RecoverPanic("unit", nil)
		panic("boom")
	}()

	if logs := panicLogs(t, workdir); len(logs) != 1 {
		t.Fatalf("expected one crash log in the working directory, found %d", len(logs))
	}
}

// The cleanup function is how callers release whatever the panicking goroutine
// owned, so it must run even when the log file cannot be written.
func TestRecoverPanicRunsCleanupWhenLogCannotBeWritten(t *testing.T) {
	t.Chdir(t.TempDir())

	// A regular file where a directory is expected: MkdirAll cannot succeed.
	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, nil, 0o600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	old := PanicDir
	PanicDir = filepath.Join(blocked, "state")
	t.Cleanup(func() { PanicDir = old })

	cleaned := false
	func() {
		defer RecoverPanic("unit", func() { cleaned = true })
		panic("boom")
	}()

	if !cleaned {
		t.Fatal("cleanup did not run when the crash log could not be written")
	}
}
