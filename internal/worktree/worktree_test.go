package worktree_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/aux-ai/aux-cli/internal/worktree"
)

func newTestRepo(t *testing.T) string {
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
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	run("add", "-A")
	run("commit", "-q", "-m", "init")
	return dir
}

func TestCreateAndRemoveWorktree(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	wtPath := filepath.Join(t.TempDir(), "subagent-1")

	wt, err := worktree.Create(ctx, repo, wtPath, "aux/subagent-1", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if wt.Path != wtPath || wt.Branch != "aux/subagent-1" {
		t.Fatalf("unexpected worktree: %+v", wt)
	}
	if _, err := os.Stat(filepath.Join(wtPath, "README.md")); err != nil {
		t.Fatalf("expected the worktree to contain the repo's tracked files: %v", err)
	}

	// The worktree is isolated: a file written there does not appear in repo.
	if err := os.WriteFile(filepath.Join(wtPath, "subagent-only.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "subagent-only.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected the parent repo to be unaffected by the worktree's file, got err=%v", err)
	}

	if err := worktree.Remove(ctx, repo, wtPath, true); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Fatalf("expected worktree path to be removed, got err=%v", err)
	}
}

func TestCreateFailsOnDuplicateBranch(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	if _, err := worktree.Create(ctx, repo, filepath.Join(t.TempDir(), "wt1"), "aux/dup", ""); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := worktree.Create(ctx, repo, filepath.Join(t.TempDir(), "wt2"), "aux/dup", ""); err == nil {
		t.Fatal("expected an error creating a worktree on an already-checked-out branch")
	}
}
