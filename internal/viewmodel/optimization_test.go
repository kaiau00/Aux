package viewmodel_test

import (
	"context"
	"testing"

	"github.com/aux-ai/aux-cli/internal/db/dbtest"
	"github.com/aux-ai/aux-cli/internal/eval"
	"github.com/aux-ai/aux-cli/internal/eventstore"
	"github.com/aux-ai/aux-cli/internal/govpolicy"
	"github.com/aux-ai/aux-cli/internal/viewmodel"
)

func TestOptimizationViewListsExperimentsAndPolicies(t *testing.T) {
	conn := dbtest.New(t)
	ctx := context.Background()
	expStore := eval.NewExperimentStore(conn)
	events := eventstore.NewService(conn)
	policySvc := govpolicy.NewService(govpolicy.NewStore(conn), events)

	exp, _, err := eval.RunCompilerExperiment(ctx, expStore, "proj-1")
	if err != nil {
		t.Fatalf("RunCompilerExperiment: %v", err)
	}
	_ = exp

	if _, err := policySvc.Candidate(ctx, "project", "proj-1", "bug_diagnosis", `{"mode":"efficient"}`); err != nil {
		t.Fatalf("policy Candidate: %v", err)
	}

	stores := viewmodel.OptimizationStores{Experiments: expStore, Policies: policySvc}
	view, err := stores.OptimizationView(ctx, "proj-1")
	if err != nil {
		t.Fatalf("OptimizationView: %v", err)
	}
	if len(view.Experiments) != 1 || view.Experiments[0].Runs != 3 {
		t.Fatalf("expected 1 experiment with 3 runs, got %+v", view.Experiments)
	}
	if len(view.CandidatePolicies) != 1 {
		t.Fatalf("expected 1 candidate policy, got %+v", view.CandidatePolicies)
	}
}

func TestOptimizationViewWithNilReadersIsEmpty(t *testing.T) {
	view, err := viewmodel.OptimizationStores{}.OptimizationView(context.Background(), "proj-1")
	if err != nil {
		t.Fatalf("OptimizationView: %v", err)
	}
	if len(view.Experiments) != 0 || len(view.ActivePolicies) != 0 {
		t.Fatalf("expected empty view with nil readers, got %+v", view)
	}
}
