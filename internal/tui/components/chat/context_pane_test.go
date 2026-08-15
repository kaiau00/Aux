package chat

import (
	"context"
	"testing"

	"github.com/aux-ai/aux-cli/internal/app"
	"github.com/aux-ai/aux-cli/internal/contextstore"
	"github.com/aux-ai/aux-cli/internal/db/dbtest"
	"github.com/aux-ai/aux-cli/internal/viewmodel"
)

func TestToggleCrossPersistsRealExclusion(t *testing.T) {
	conn := dbtest.New(t)
	pages := contextstore.NewStore(conn)
	m := NewContextPaneCmp(&app.App{Pages: pages})
	m.taskID = "task-1"
	m.entries = []ContextEntry{{Path: "a.go", ToolCallID: "call-a"}}
	m.selected = 0

	m.toggleCross(true)

	excl, err := pages.Exclusions(context.Background(), "task-1")
	if err != nil {
		t.Fatalf("Exclusions: %v", err)
	}
	if !excl["call-a"] {
		t.Fatalf("expected call-a excluded after cross-off, got %+v", excl)
	}
	if !m.entries[0].CrossedOff {
		t.Fatal("expected the local entry to also show crossed off")
	}

	m.toggleCross(false)
	excl, err = pages.Exclusions(context.Background(), "task-1")
	if err != nil {
		t.Fatalf("Exclusions after un-cross: %v", err)
	}
	if excl["call-a"] {
		t.Fatalf("expected call-a no longer excluded after un-cross, got %+v", excl)
	}
}

func TestClearCrossedClearsAllExclusions(t *testing.T) {
	conn := dbtest.New(t)
	pages := contextstore.NewStore(conn)
	m := NewContextPaneCmp(&app.App{Pages: pages})
	m.taskID = "task-1"
	m.entries = []ContextEntry{
		{Path: "a.go", ToolCallID: "call-a", CrossedOff: true},
		{Path: "b.go", ToolCallID: "call-b", CrossedOff: false},
	}
	if err := pages.Exclude(context.Background(), "task-1", "call-a"); err != nil {
		t.Fatalf("seed Exclude: %v", err)
	}

	m.clearCrossed()

	if len(m.entries) != 1 || m.entries[0].ToolCallID != "call-b" {
		t.Fatalf("expected only the non-crossed entry to remain, got %+v", m.entries)
	}
	excl, err := pages.Exclusions(context.Background(), "task-1")
	if err != nil {
		t.Fatalf("Exclusions: %v", err)
	}
	if len(excl) != 0 {
		t.Fatalf("expected all exclusions cleared, got %+v", excl)
	}
}

func TestExpandKeyTogglesExpandedView(t *testing.T) {
	m := NewContextPaneCmp(&app.App{})
	m.width = 60
	m.budget.TotalTokens = 100
	m.budget.Categories = []viewmodel.ContextCategoryVM{{Label: "Active code", Tokens: 100}}
	m.pageList.Resident = []viewmodel.ContextPageEntryVM{{StableKey: "file:/a.go", Tokens: 100}}

	compact := m.budgetView()
	if compact == "" {
		t.Fatal("expected the compact budget view to render")
	}

	m.expandedContext = true
	expanded := m.budgetView()
	if expanded == "" {
		t.Fatal("expected the expanded view to render once toggled")
	}
	if expanded == compact {
		t.Fatal("expanded view should differ from the compact view")
	}
}

func TestExpandedViewFallsBackToCompactWhenNoPageBindings(t *testing.T) {
	m := NewContextPaneCmp(&app.App{})
	m.width = 60
	m.budget.TotalTokens = 100
	m.budget.Categories = []viewmodel.ContextCategoryVM{{Label: "Active code", Tokens: 100}}
	m.expandedContext = true // no pageList populated

	if got := m.budgetView(); got == "" {
		t.Fatal("expected a fallback to the compact view when there is nothing to expand")
	}
}

func TestToggleCrossWithoutTaskDoesNotPanic(t *testing.T) {
	m := NewContextPaneCmp(&app.App{})
	m.entries = []ContextEntry{{Path: "a.go", ToolCallID: "call-a"}}
	m.selected = 0
	m.toggleCross(true) // no Pages, no taskID: must be a safe no-op beyond local state
	if !m.entries[0].CrossedOff {
		t.Fatal("local state should still update even without a wired store")
	}
}
