package agent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func newTestGitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "committed.txt"), []byte("committed\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	run("add", "-A")
	run("commit", "-q", "-m", "init")
	return dir
}

func TestPrepareValidationWorktreeIsolatesAndSyncsLiveState(t *testing.T) {
	repo := newTestGitRepo(t)
	// Dirty the repo after the commit: prepareValidationWorktree must reflect
	// this uncommitted edit, not just the last commit (validation checking
	// stale code would be worse than not isolating at all).
	if err := os.WriteFile(filepath.Join(repo, "committed.txt"), []byte("edited\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	dir, _, cleanup, ok := prepareValidationWorktree(context.Background(), repo, "call-1")
	if !ok {
		t.Fatal("expected worktree preparation to succeed in a real git repo")
	}

	if dir == repo {
		t.Fatal("worktree dir must be isolated from the parent repo root")
	}
	got, err := os.ReadFile(filepath.Join(dir, "committed.txt"))
	if err != nil || string(got) != "edited\n" {
		t.Fatalf("expected the worktree to reflect the parent's uncommitted edit, got %q err=%v", got, err)
	}

	// Writing inside the worktree must not leak back into the parent repo.
	if err := os.WriteFile(filepath.Join(dir, "subagent-only.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "subagent-only.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected the parent repo to be unaffected by the worktree's file, got err=%v", err)
	}

	cleanup()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("expected cleanup to remove the worktree, got err=%v", err)
	}
}

func TestTurnWriteTrackerFlagsOverlapWithinATurn(t *testing.T) {
	tr := newTurnWriteTracker()

	if got := tr.RecordAndCheck("turn-1", []string{"a.go", "b.go"}); len(got) != 0 {
		t.Fatalf("first subtask in a turn should see no conflicts, got %v", got)
	}
	got := tr.RecordAndCheck("turn-1", []string{"b.go", "c.go"})
	if len(got) != 1 || got[0] != "b.go" {
		t.Fatalf("expected exactly [b.go] to conflict, got %v", got)
	}

	// A different turn is an independent scope: no false-positive overlap.
	if got := tr.RecordAndCheck("turn-2", []string{"b.go"}); len(got) != 0 {
		t.Fatalf("a different turn must not see turn-1's writes, got %v", got)
	}
}

func TestTurnWriteTrackerIgnoresEmptyTurnID(t *testing.T) {
	tr := newTurnWriteTracker()
	tr.RecordAndCheck("", []string{"a.go"})
	if got := tr.RecordAndCheck("", []string{"a.go"}); len(got) != 0 {
		t.Fatalf("an empty turn id must never be tracked or conflict, got %v", got)
	}
}

func TestPrepareValidationWorktreeFallsBackWhenNotAGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	notARepo := t.TempDir()

	dir, _, cleanup, ok := prepareValidationWorktree(context.Background(), notARepo, "call-2")
	if ok {
		t.Fatalf("expected a graceful failure outside a git repo, got dir=%q", dir)
	}
	if dir != "" {
		t.Fatalf("expected an empty dir on failure, got %q", dir)
	}
	cleanup() // must be a safe no-op
}
