package eval_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aux-ai/aux-cli/internal/db/dbtest"
	"github.com/aux-ai/aux-cli/internal/eval"
	"github.com/aux-ai/aux-cli/internal/eventstore"
)

func TestRunCompilerExperimentPersists(t *testing.T) {
	store := eval.NewExperimentStore(dbtest.New(t))
	ctx := context.Background()

	exp, results, err := eval.RunCompilerExperiment(ctx, store, "proj-1")
	if err != nil {
		t.Fatalf("RunCompilerExperiment: %v", err)
	}
	if exp.ID == "" || len(results) != 3 {
		t.Fatalf("expected an experiment with 3 fixture results, got %d", len(results))
	}

	runs, err := store.ListRuns(ctx, exp.ID)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 3 {
		t.Fatalf("expected 3 persisted eval runs, got %d", len(runs))
	}
	// Every run is lossless -> status pass; identifies the variant.
	for _, r := range runs {
		if r.Status != "pass" {
			t.Fatalf("run %s should be pass (lossless), got %q", r.EvalCaseID, r.Status)
		}
		if r.Variant != "paging" {
			t.Fatalf("variant should be paging, got %q", r.Variant)
		}
	}

	got, err := store.GetExperiment(ctx, exp.ID)
	if err != nil || got.Name == "" {
		t.Fatalf("experiment not retrievable: %v", err)
	}
}

func TestReplayTaskState(t *testing.T) {
	events := []eventstore.Event{
		{Type: eventstore.TaskCreated},
		{Type: eventstore.TaskStarted},
		{Type: eventstore.TurnStarted},
		{Type: eventstore.ModelCallStarted},
		{Type: eventstore.ToolStarted},
		{Type: eventstore.ToolFailed},
		{Type: eventstore.TaskCompleted},
	}
	rt := eval.ReplayTaskState("t1", events)
	if rt.Status != "completed" {
		t.Fatalf("status = %q, want completed", rt.Status)
	}
	if rt.Turns != 1 || rt.ModelCalls != 1 || rt.ToolCalls != 1 || rt.Failures != 1 {
		t.Fatalf("replay counts wrong: %+v", rt)
	}
	if rt.Validated {
		t.Fatalf("should not be validated without a passed validation event")
	}
}

func TestReplayValidated(t *testing.T) {
	raw, err := json.Marshal(eventstore.ValidationPayload{Status: "passed"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	rt := eval.ReplayTaskState("t2", []eventstore.Event{
		{Type: eventstore.TaskCompleted},
		{Type: eventstore.ValidationCompleted, Payload: raw},
	})
	if !rt.Validated {
		t.Fatalf("expected validated=true when a validation passed")
	}
}
