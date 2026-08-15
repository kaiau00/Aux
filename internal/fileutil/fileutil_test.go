package fileutil

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestGetRgCmdDoesNotFollowSymlinks locks in a fix for a real incident: rg was
// invoked with -L (follow symlinks) and no bound on results or run time. Run
// from a directory that reaches a large or cyclical tree via a symlink (a
// cloud-sync placeholder mount, in the incident that motivated this test),
// that combination can enumerate an unbounded number of paths, exhausting
// memory. rg must never be asked to follow symlinks here.
func TestGetRgCmdDoesNotFollowSymlinks(t *testing.T) {
	saved := rgPath
	rgPath = "/usr/bin/true" // any path so GetRgCmd doesn't short-circuit to nil
	defer func() { rgPath = saved }()

	cmd := GetRgCmd(context.Background(), "")
	if cmd == nil {
		t.Fatal("GetRgCmd returned nil")
	}
	if slices.Contains(cmd.Args, "-L") || slices.Contains(cmd.Args, "--follow") {
		t.Fatalf("rg must not follow symlinks (found -L/--follow in args): %v", cmd.Args)
	}
}

func TestGetRgCmdNilWhenNotFound(t *testing.T) {
	saved := rgPath
	rgPath = ""
	defer func() { rgPath = saved }()

	if cmd := GetRgCmd(context.Background(), ""); cmd != nil {
		t.Fatalf("expected nil when rg is not on PATH, got %v", cmd.Args)
	}
}

func TestGetFzfCmdNilWhenNotFound(t *testing.T) {
	saved := fzfPath
	fzfPath = ""
	defer func() { fzfPath = saved }()

	if cmd := GetFzfCmd(context.Background(), "query"); cmd != nil {
		t.Fatalf("expected nil when fzf is not on PATH, got %v", cmd.Args)
	}
}

// TestGlobWithDoublestarLimitBoundsResults guards against the same class of
// unbounded-enumeration problem on the non-rg fallback path: a limit of 0 must
// not silently mean "unlimited" for callers that pass a real bound.
func TestGlobWithDoublestarLimitBoundsResults(t *testing.T) {
	dir := t.TempDir()
	for i := range 10 {
		if err := os.WriteFile(filepath.Join(dir, "file"+string(rune('a'+i))+".txt"), []byte("x"), 0o644); err != nil {
			t.Fatalf("write fixture file: %v", err)
		}
	}

	matches, truncated, err := GlobWithDoublestar("**/*", dir, 3)
	if err != nil {
		t.Fatalf("GlobWithDoublestar: %v", err)
	}
	if len(matches) != 3 {
		t.Fatalf("expected 3 matches with limit=3, got %d: %v", len(matches), matches)
	}
	if !truncated {
		t.Fatal("expected truncated=true when more files exist than the limit")
	}
}
