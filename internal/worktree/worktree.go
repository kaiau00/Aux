// Package worktree creates and removes isolated git worktrees so a subagent's
// file changes land on their own branch and directory rather than directly on
// the parent's working tree (roadmapplan.md §11.3). It is a thin wrapper over
// the `git worktree` porcelain — deterministic and testable without a fake,
// the same choice internal/project makes for VCS inspection (see
// internal/project/vcs.go).
package worktree

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Worktree is a created git worktree: an isolated working directory checked
// out on its own branch.
type Worktree struct {
	Path   string
	Branch string
}

// Create adds a new git worktree at path, checked out on a new branch created
// from base ("HEAD" when base is empty). repoRoot must be inside a git working
// tree; path must not already exist.
func Create(ctx context.Context, repoRoot, path, branch, base string) (Worktree, error) {
	if base == "" {
		base = "HEAD"
	}
	if err := run(ctx, repoRoot, "worktree", "add", "-b", branch, path, base); err != nil {
		return Worktree{}, fmt.Errorf("failed to create worktree %q on branch %q: %w", path, branch, err)
	}
	return Worktree{Path: path, Branch: branch}, nil
}

// Remove removes a worktree. force also removes one with uncommitted or
// unmerged changes, which is expected once a subagent's result has been
// reported back to the parent and the worktree is being torn down regardless
// of what it left behind.
func Remove(ctx context.Context, repoRoot, path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)
	if err := run(ctx, repoRoot, args...); err != nil {
		return fmt.Errorf("failed to remove worktree %q: %w", path, err)
	}
	return nil
}

func run(ctx context.Context, dir string, args ...string) error {
	full := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(ctx, "git", full...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
