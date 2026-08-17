package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

const (
	ContextExcludeToolName = "context_exclude"

	contextExcludeDescription = `Drop files you have already read from the context of subsequent turns.

Use this when a file you read turns out to be irrelevant, or once you have taken
what you need from it and no longer need its full text in front of you. Excluded
files stop being sent to the model on the next turn, which leaves more room for
the work that matters. The exclusion applies to the current task only; nothing is
deleted from disk and you can read the file again later if you need to.

Usage notes:
1. Pass the same paths you saw in the read results. Relative and absolute paths
   both work.
2. Excluding a file you never read is a no-op and reports as such — it is not an
   error.
3. Prefer excluding a few large, clearly-finished files over excluding
   everything you touch; the user can see and undo these decisions.`
)

// PageStore is the subset of the context page store this tool needs. Declared
// here rather than imported so the tools package keeps its existing shape and
// does not take on a store dependency it cannot test without a database.
type PageStore interface {
	Exclude(ctx context.Context, taskID, toolCallID string) error
}

// ReadHistory supplies the tool results of a session, which is where the
// path-to-tool-call mapping lives.
type ReadHistory interface {
	ToolCallIDsForPaths(ctx context.Context, sessionID string, paths []string) (map[string][]string, error)
}

type ContextExcludeParams struct {
	Paths []string `json:"paths"`
	// Reason is recorded for the user's benefit, not used for matching.
	Reason string `json:"reason,omitempty"`
}

type ContextExcludeResponseMetadata struct {
	Excluded []string `json:"excluded"`
	NotFound []string `json:"not_found"`
	Reason   string   `json:"reason,omitempty"`
}

type contextExcludeTool struct {
	pages   PageStore
	history ReadHistory
}

// NewContextExcludeTool returns the tool that lets the model drop its own
// context pages.
//
// Per-page exclusion already existed and was already honoured by the compiler,
// but only the TUI could set it — the model had no way to reach a feature built
// for exactly the situation it is in most often, having just read something it
// did not need.
func NewContextExcludeTool(pages PageStore, history ReadHistory) BaseTool {
	return &contextExcludeTool{pages: pages, history: history}
}

func (t *contextExcludeTool) Info() ToolInfo {
	return ToolInfo{
		Name:        ContextExcludeToolName,
		Description: contextExcludeDescription,
		Parameters: map[string]any{
			"paths": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Paths of already-read files to drop from context",
			},
			"reason": map[string]any{
				"type":        "string",
				"description": "Short note on why these are no longer needed",
			},
		},
		Required: []string{"paths"},
	}
}

func (t *contextExcludeTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params ContextExcludeParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse("invalid input: " + err.Error()), nil
	}
	if len(params.Paths) == 0 {
		return NewTextErrorResponse("paths is required"), nil
	}
	if t.pages == nil || t.history == nil {
		return NewTextErrorResponse("context exclusion is not available in this session"), nil
	}

	corr := CorrelationFromContext(ctx)
	if corr.TaskID == "" || corr.SessionID == "" {
		return NewTextErrorResponse("context exclusion needs an active task"), nil
	}

	matches, err := t.history.ToolCallIDsForPaths(ctx, corr.SessionID, params.Paths)
	if err != nil {
		return ToolResponse{}, fmt.Errorf("failed to resolve read history: %w", err)
	}

	var excluded, notFound []string
	for _, p := range params.Paths {
		ids := matches[p]
		if len(ids) == 0 {
			notFound = append(notFound, p)
			continue
		}
		for _, id := range ids {
			if err := t.pages.Exclude(ctx, corr.TaskID, id); err != nil {
				return ToolResponse{}, fmt.Errorf("failed to exclude %s: %w", p, err)
			}
		}
		excluded = append(excluded, p)
	}
	sort.Strings(excluded)
	sort.Strings(notFound)

	meta := ContextExcludeResponseMetadata{
		Excluded: excluded,
		NotFound: notFound,
		Reason:   params.Reason,
	}

	var b strings.Builder
	if len(excluded) > 0 {
		fmt.Fprintf(&b, "Dropped from context: %s", strings.Join(excluded, ", "))
	}
	if len(notFound) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "Not in context, nothing to drop: %s", strings.Join(notFound, ", "))
	}
	return WithResponseMetadata(NewTextResponse(b.String()), meta), nil
}

// SamePath reports whether two paths refer to the same file, tolerating the mix
// of absolute and workdir-relative forms that flows through tool results and
// model-supplied arguments.
//
// It is lexical on purpose: it runs against recorded history, where the file may
// have since been moved or deleted, so it must not depend on the path still
// resolving on disk.
func SamePath(workDir, a, b string) bool {
	return normalizePath(workDir, a) == normalizePath(workDir, b)
}

func normalizePath(workDir, p string) string {
	p = filepath.Clean(strings.TrimSpace(p))
	if !filepath.IsAbs(p) && workDir != "" {
		p = filepath.Join(workDir, p)
	}
	return p
}
