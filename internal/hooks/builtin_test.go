package hooks

import (
	"context"
	"testing"
)

func TestRegisterObservabilityCoversEveryPoint(t *testing.T) {
	// The gap this closes: dispatch points existed with no handler registered
	// anywhere in production, so every Dispatch ran an empty list.
	r := NewRegistry()
	RegisterObservability(r)

	for _, p := range []Point{TaskBegin, TaskEnd, SubtaskBegin, SubtaskEnd, ToolPre, ToolPost, ValidationComplete} {
		if r.Count(p) == 0 {
			t.Errorf("lifecycle point %q has no registered handler", p)
		}
	}
}

func TestObservabilityHandlersNeverVeto(t *testing.T) {
	// An error from a ToolPre handler vetoes the tool call, so observation
	// must never be able to block real work.
	r := NewRegistry()
	RegisterObservability(r)

	for _, p := range []Point{TaskBegin, TaskEnd, SubtaskBegin, SubtaskEnd, ToolPre, ToolPost, ValidationComplete} {
		if err := r.Dispatch(context.Background(), Event{Point: p, Tool: "bash", TaskID: "t1"}); err != nil {
			t.Errorf("point %q: observability handler must not veto, got %v", p, err)
		}
	}
}

func TestRegisterObservabilityNilRegistryIsSafe(t *testing.T) {
	RegisterObservability(nil) // must not panic
}
