package validation_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aux-ai/aux-cli/internal/db/dbtest"
	"github.com/aux-ai/aux-cli/internal/eventstore"
	"github.com/aux-ai/aux-cli/internal/validation"
)

// fakeRunner returns scripted results and counts invocations.
type fakeRunner struct {
	exit  int
	err   error
	calls int
}

func (f *fakeRunner) Run(context.Context, string) (validation.CommandResult, error) {
	f.calls++
	return validation.CommandResult{ExitCode: f.exit, DurationMS: 5}, f.err
}

func newService(t *testing.T) (*validation.Service, *validation.Store) {
	t.Helper()
	conn := dbtest.New(t)
	store := validation.NewStore(conn)
	return validation.NewService(store, eventstore.NewService(conn)), store
}

func TestRunIntentPassAdvancesCriterion(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()
	runner := &fakeRunner{exit: 0}

	intent := validation.Intent{ID: "i1", ValidatorType: "go.test", Command: "go test ./...", CriterionIDs: []string{"c1"}}
	res, err := svc.RunIntent(ctx, "task1", intent, "fp-1", runner)
	if err != nil {
		t.Fatalf("RunIntent: %v", err)
	}
	if res.Run.Status != validation.StatusPassed {
		t.Fatalf("status = %q, want passed", res.Run.Status)
	}

	states, err := svc.ProofOfDone(ctx, "task1", []string{"c1"})
	if err != nil {
		t.Fatalf("ProofOfDone: %v", err)
	}
	if states["c1"] != validation.Validated {
		t.Fatalf("criterion should be validated, got %q", states["c1"])
	}
}

func TestRunIntentFailBlocksCriterion(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()
	runner := &fakeRunner{exit: 1}

	intent := validation.Intent{ID: "i1", Command: "go test ./...", CriterionIDs: []string{"c1"}}
	res, _ := svc.RunIntent(ctx, "task1", intent, "fp-1", runner)
	if res.Run.Status != validation.StatusFailed {
		t.Fatalf("status = %q, want failed", res.Run.Status)
	}
	states, _ := svc.ProofOfDone(ctx, "task1", []string{"c1"})
	if states["c1"] != validation.Blocked {
		t.Fatalf("failed validation should block the criterion, got %q", states["c1"])
	}
}

func TestUncoveredCriterionIsUncovered(t *testing.T) {
	svc, _ := newService(t)
	states, _ := svc.ProofOfDone(context.Background(), "task1", []string{"never-evidenced"})
	if states["never-evidenced"] != validation.Uncovered {
		t.Fatalf("criterion with no evidence should be uncovered, got %q", states["never-evidenced"])
	}
}

func TestCacheReusesOnlyWhenInputUnchanged(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()
	intent := validation.Intent{ID: "i1", Command: "go test ./...", CriterionIDs: []string{"c1"}}

	// First run executes.
	r1 := &fakeRunner{exit: 0}
	res1, _ := svc.RunIntent(ctx, "task1", intent, "fp-A", r1)
	if res1.Cached || r1.calls != 1 {
		t.Fatalf("first run should execute, not cache")
	}

	// Same command + same input fingerprint -> cached, runner not called.
	r2 := &fakeRunner{exit: 0}
	res2, _ := svc.RunIntent(ctx, "task1", intent, "fp-A", r2)
	if !res2.Cached || r2.calls != 0 {
		t.Fatalf("unchanged inputs should reuse the cached pass (calls=%d cached=%v)", r2.calls, res2.Cached)
	}

	// Changed input fingerprint -> must re-run (never reuse across changed inputs).
	r3 := &fakeRunner{exit: 0}
	res3, _ := svc.RunIntent(ctx, "task1", intent, "fp-B", r3)
	if res3.Cached || r3.calls != 1 {
		t.Fatalf("changed inputs must re-run, not reuse (calls=%d cached=%v)", r3.calls, res3.Cached)
	}
}

func TestNoCacheWithoutFingerprint(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()
	intent := validation.Intent{ID: "i1", Command: "go test ./...", CriterionIDs: []string{"c1"}}
	svc.RunIntent(ctx, "t", intent, "", &fakeRunner{exit: 0})
	r := &fakeRunner{exit: 0}
	res, _ := svc.RunIntent(ctx, "t", intent, "", r)
	if res.Cached || r.calls != 1 {
		t.Fatalf("without a fingerprint a passing result must not be reused")
	}
}

func TestWaiverAndSuccessfulCommands(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()

	// A waived criterion.
	_ = svc.WaiveCriterion(ctx, "task1", "c1", "not applicable")
	states, _ := svc.ProofOfDone(ctx, "task1", []string{"c1"})
	if states["c1"] != validation.WaivedByUser {
		t.Fatalf("expected waived_by_user, got %q", states["c1"])
	}

	// A passing command shows up in SuccessfulCommands.
	_, _ = svc.RunIntent(ctx, "task1", validation.Intent{ID: "i", Command: "go vet ./...", CriterionIDs: []string{"c2"}}, "fp", &fakeRunner{exit: 0})
	cmds, err := svc.SuccessfulCommands(ctx, "task1")
	if err != nil {
		t.Fatalf("SuccessfulCommands: %v", err)
	}
	if len(cmds) != 1 || cmds[0] != "go vet ./..." {
		t.Fatalf("expected [go vet ./...], got %v", cmds)
	}
}

func TestRunnerErrorIsFailed(t *testing.T) {
	svc, _ := newService(t)
	res, _ := svc.RunIntent(context.Background(), "t", validation.Intent{ID: "i", Command: "x"}, "fp", &fakeRunner{err: errors.New("boom")})
	if res.Run.Status != validation.StatusFailed {
		t.Fatalf("runner error should mark run failed")
	}
}
