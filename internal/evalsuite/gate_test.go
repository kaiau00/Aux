package evalsuite

import (
	"strings"
	"testing"
)

// run builds a SuiteRun from compact per-task specs.
func run(specs ...Run) SuiteRun {
	return SuiteRun{Suite: "test", Runs: specs}
}

func task(id string, ok bool, tokens, turns int64) Run {
	return Run{TaskID: id, Succeeded: ok, InputTokens: tokens, Turns: turns}
}

// The rule the whole harness exists to enforce. Halving token use while solving
// fewer tasks is not an improvement, and no token saving may buy its way past
// this.
func TestCheaperButWorseFailsTheGate(t *testing.T) {
	baseline := run(
		task("a", true, 1000, 10),
		task("b", true, 1000, 10),
		task("c", true, 1000, 10),
	)
	candidate := run(
		task("a", true, 100, 3),
		task("b", true, 100, 3),
		task("c", false, 100, 3), // solved one fewer
	)

	v := Gate(baseline, candidate, DefaultThresholds())
	if v.Passed {
		t.Fatal("a candidate that solves fewer tasks must fail regardless of token savings")
	}
	if len(v.Failures) == 0 {
		t.Fatal("a failing verdict must say why")
	}
	joined := strings.Join(v.Failures, " ")
	if !strings.Contains(joined, "success rate") {
		t.Fatalf("the reason should name the success rate, got %v", v.Failures)
	}
	if len(v.Regressions) != 1 || v.Regressions[0] != "c" {
		t.Fatalf("the regressed task should be named, got %v", v.Regressions)
	}
}

// An aggregate success rate can hold steady while the set of working tasks
// churns underneath it. That is a behaviour change worth a human looking at.
func TestChurnedOutcomesFailEvenAtEqualSuccessRate(t *testing.T) {
	baseline := run(
		task("a", true, 1000, 10),
		task("b", false, 1000, 10),
	)
	candidate := run(
		task("a", false, 500, 5), // broke
		task("b", true, 500, 5),  // fixed
	)

	v := Gate(baseline, candidate, DefaultThresholds())
	if v.Baseline.SuccessRate != v.Candidate.SuccessRate {
		t.Fatal("this test is only meaningful when the aggregate rate is unchanged")
	}
	if v.Passed {
		t.Fatal("a task that regressed must fail the gate even when the rate is level")
	}
	if len(v.Regressions) != 1 || v.Regressions[0] != "a" {
		t.Fatalf("expected 'a' as the regression, got %v", v.Regressions)
	}
	if len(v.Fixes) != 1 || v.Fixes[0] != "b" {
		t.Fatalf("expected 'b' as a fix, got %v", v.Fixes)
	}
}

func TestGenuineImprovementPasses(t *testing.T) {
	baseline := run(task("a", true, 1000, 10), task("b", true, 1000, 10))
	candidate := run(task("a", true, 600, 10), task("b", true, 600, 10))

	v := Gate(baseline, candidate, DefaultThresholds())
	if !v.Passed {
		t.Fatalf("same success rate and 40%% fewer tokens should pass, got %v", v.Failures)
	}
	if len(v.Regressions) != 0 {
		t.Fatalf("no regressions expected, got %v", v.Regressions)
	}
}

// Missing the savings target while regressing nothing is worth reporting, not
// worth blocking: the change still made things better.
func TestMissingTheTokenTargetIsANoteNotAFailure(t *testing.T) {
	baseline := run(task("a", true, 1000, 10))
	candidate := run(task("a", true, 900, 10)) // 10% saved, target is 30%

	v := Gate(baseline, candidate, DefaultThresholds())
	if !v.Passed {
		t.Fatalf("an improvement short of target should not fail, got %v", v.Failures)
	}
	if len(v.Notes) == 0 {
		t.Fatal("missing the target should be reported as a note")
	}
	if !strings.Contains(strings.Join(v.Notes, " "), "missed") {
		t.Fatalf("the note should say the target was missed, got %v", v.Notes)
	}
}

func TestRisingTokensFail(t *testing.T) {
	baseline := run(task("a", true, 1000, 10))
	candidate := run(task("a", true, 1200, 10))

	if v := Gate(baseline, candidate, DefaultThresholds()); v.Passed {
		t.Fatal("more tokens for the same result must fail")
	}
}

// A large turn increase is thrashing, even when tokens fall.
func TestThrashingFailsDespiteTokenSavings(t *testing.T) {
	baseline := run(task("a", true, 1000, 10))
	candidate := run(task("a", true, 400, 25)) // 60% fewer tokens, 2.5x the turns

	v := Gate(baseline, candidate, DefaultThresholds())
	if v.Passed {
		t.Fatal("a large turn increase must fail even with a big token saving")
	}
	if !strings.Contains(strings.Join(v.Failures, " "), "turns") {
		t.Fatalf("the reason should name turns, got %v", v.Failures)
	}
}

func TestSmallTurnIncreaseIsAllowed(t *testing.T) {
	baseline := run(task("a", true, 1000, 10))
	candidate := run(task("a", true, 500, 11)) // within the 1.1x ceiling

	if v := Gate(baseline, candidate, DefaultThresholds()); !v.Passed {
		t.Fatalf("trading one extra turn for half the tokens should pass, got %v", v.Failures)
	}
}

// The most dangerous possible outcome: a suite that ran nothing reporting a
// pass, which would be read as evidence.
func TestEmptyRunsNeverPass(t *testing.T) {
	full := run(task("a", true, 100, 1))

	for _, tc := range []struct {
		name                string
		baseline, candidate SuiteRun
	}{
		{"both empty", run(), run()},
		{"candidate empty", full, run()},
		{"baseline empty", run(), full},
	} {
		if v := Gate(tc.baseline, tc.candidate, DefaultThresholds()); v.Passed {
			t.Errorf("%s: an empty comparison must not pass", tc.name)
		}
	}
}

// Tasks added or removed between runs are not regressions; the totals report
// the change instead.
func TestAddedAndRemovedTasksAreNotRegressions(t *testing.T) {
	baseline := run(task("a", true, 500, 5), task("gone", true, 500, 5))
	candidate := run(task("a", true, 300, 5), task("new", false, 100, 2))

	v := Gate(baseline, candidate, DefaultThresholds())
	if len(v.Regressions) != 0 {
		t.Fatalf("a removed task is not a regression, got %v", v.Regressions)
	}
	// The new failing task still lowers the success rate, which does fail.
	if v.Passed {
		t.Fatal("a newly failing task lowers the success rate and must fail")
	}
}

func TestSummarizeAggregatesCorrectly(t *testing.T) {
	sr := run(
		Run{TaskID: "a", Succeeded: true, InputTokens: 100, OutputTokens: 50, Turns: 3, Cost: 0.10, DurationMS: 1000},
		Run{TaskID: "b", Succeeded: false, InputTokens: 200, OutputTokens: 100, Turns: 5, Cost: 0.20, DurationMS: 2000, CostUnknown: true},
	)
	s := sr.Summarize()

	if s.Total != 2 || s.Succeeded != 1 || s.SuccessRate != 0.5 {
		t.Fatalf("unexpected counts: %+v", s)
	}
	if s.TotalTokens != 450 {
		t.Fatalf("tokens should sum input and output across tasks, got %d", s.TotalTokens)
	}
	if s.TotalTurns != 8 {
		t.Fatalf("turns should sum, got %d", s.TotalTurns)
	}
	if !s.CostUnknown {
		t.Fatal("one unknown-cost run must taint the aggregate, or an unmeasurable cost reads as a cheap one")
	}
}

// Reporting an unchanged number as a saving is the overstatement this gate
// exists to prevent, so "unchanged" and "fell short of target" are distinct.
func TestUnchangedTokensAreNotDescribedAsFalling(t *testing.T) {
	same := run(task("a", true, 1000, 10))

	v := Gate(same, same, DefaultThresholds())
	if !v.Passed {
		t.Fatalf("an identical run should pass, got %v", v.Failures)
	}
	notes := strings.Join(v.Notes, " ")
	if strings.Contains(notes, "fell") {
		t.Fatalf("identical token counts must not be reported as a reduction: %q", notes)
	}
	if !strings.Contains(notes, "unchanged") {
		t.Fatalf("expected the note to say tokens were unchanged, got %q", notes)
	}
}
