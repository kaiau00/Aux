package evalsuite

import (
	"fmt"
	"sort"
)

// Default gate thresholds (TODO.md P0.1).
const (
	// DefaultTokenRatio is the token budget as a fraction of baseline: the
	// savings a change must actually deliver to count.
	DefaultTokenRatio = 0.7

	// DefaultTurnRatio allows a small increase in turns. Trading a few extra
	// cheap turns for a large token saving is a real win; a large increase is
	// thrashing wearing a saving as a disguise.
	DefaultTurnRatio = 1.1
)

// Thresholds parameterizes the gate. Success rate has no threshold field on
// purpose: it is a floor, not a budget.
type Thresholds struct {
	TokenRatio float64
	TurnRatio  float64
}

// DefaultThresholds returns the criteria recorded in TODO.md.
func DefaultThresholds() Thresholds {
	return Thresholds{TokenRatio: DefaultTokenRatio, TurnRatio: DefaultTurnRatio}
}

// Verdict is the outcome of gating a candidate against a baseline.
type Verdict struct {
	Passed bool `json:"passed"`

	// Failures are the criteria that were not met, in severity order.
	Failures []string `json:"failures,omitempty"`
	// Notes are observations that do not block, such as a token target missed
	// while nothing regressed.
	Notes []string `json:"notes,omitempty"`

	// Regressions names tasks that passed on the baseline and fail now. These
	// are listed explicitly because an aggregate success rate can stay level
	// while the set of working tasks churns underneath it.
	Regressions []string `json:"regressions,omitempty"`
	// Fixes names tasks that failed on the baseline and pass now.
	Fixes []string `json:"fixes,omitempty"`

	Baseline  Summary `json:"baseline"`
	Candidate Summary `json:"candidate"`
}

// Gate decides whether a candidate run may ship.
//
// The ordering here is the whole point. Success rate is a hard floor evaluated
// first and independently: a candidate that halves token use while solving
// fewer tasks has not made the harness cheaper, it has made it worse and
// cheaper, and those are not the same result. Token and turn budgets are only
// consulted once capability is known not to have regressed.
//
// Per-task regressions fail the gate even when the aggregate rate holds. Two
// tasks swapping pass for fail nets to zero in a success rate and is plainly a
// change in behaviour that someone should look at.
func Gate(baseline, candidate SuiteRun, th Thresholds) Verdict {
	v := Verdict{
		Baseline:  baseline.Summarize(),
		Candidate: candidate.Summarize(),
	}
	v.Regressions, v.Fixes = diffOutcomes(baseline, candidate)

	// 1. Capability floor.
	if v.Candidate.SuccessRate < v.Baseline.SuccessRate {
		v.Failures = append(v.Failures, fmt.Sprintf(
			"success rate fell from %.1f%% to %.1f%%: a cheaper run that solves fewer tasks is not an improvement",
			v.Baseline.SuccessRate*100, v.Candidate.SuccessRate*100))
	}
	if len(v.Regressions) > 0 {
		v.Failures = append(v.Failures, fmt.Sprintf(
			"%d task(s) that passed on the baseline now fail: %v", len(v.Regressions), v.Regressions))
	}

	// 2. Budgets, meaningful only once capability holds.
	if v.Baseline.TotalTokens > 0 {
		budget := float64(v.Baseline.TotalTokens) * th.TokenRatio
		actual := float64(v.Candidate.TotalTokens)
		switch {
		case actual > float64(v.Baseline.TotalTokens):
			v.Failures = append(v.Failures, fmt.Sprintf(
				"tokens rose from %d to %d", v.Baseline.TotalTokens, v.Candidate.TotalTokens))
		case actual == float64(v.Baseline.TotalTokens):
			// Distinguished from a real reduction on purpose: reporting an
			// unchanged number as a saving is the exact overstatement this gate
			// exists to prevent.
			v.Notes = append(v.Notes, fmt.Sprintf(
				"tokens unchanged at %d; the %.0f target was not approached",
				v.Candidate.TotalTokens, budget))
		case actual > budget:
			// Not a regression, just short of target. Saying so plainly beats
			// failing a build for a change that made things better.
			v.Notes = append(v.Notes, fmt.Sprintf(
				"tokens fell to %d but missed the %.0f target (%.0f%% of baseline, wanted %.0f%%)",
				v.Candidate.TotalTokens, budget,
				actual/float64(v.Baseline.TotalTokens)*100, th.TokenRatio*100))
		}
	}

	if v.Baseline.TotalTurns > 0 {
		ceiling := float64(v.Baseline.TotalTurns) * th.TurnRatio
		if float64(v.Candidate.TotalTurns) > ceiling {
			v.Failures = append(v.Failures, fmt.Sprintf(
				"turns rose from %d to %d, past the %.0f ceiling: this is usually thrashing rather than efficiency",
				v.Baseline.TotalTurns, v.Candidate.TotalTurns, ceiling))
		}
	}

	// An empty comparison must never read as a pass. A suite that ran nothing
	// proves nothing, and this is exactly the case where a green check would be
	// taken as evidence.
	if v.Baseline.Total == 0 || v.Candidate.Total == 0 {
		v.Failures = append(v.Failures, "one or both runs contain no tasks, so there is nothing to conclude")
	}

	v.Passed = len(v.Failures) == 0
	return v
}

// diffOutcomes reports which tasks changed pass/fail state between two runs.
// Tasks absent from either run are ignored: a suite that gained or lost tasks
// is a different measurement, and Gate reports that through the totals.
func diffOutcomes(baseline, candidate SuiteRun) (regressions, fixes []string) {
	before := make(map[string]bool, len(baseline.Runs))
	for _, r := range baseline.Runs {
		before[r.TaskID] = r.Succeeded
	}
	for _, r := range candidate.Runs {
		was, present := before[r.TaskID]
		if !present {
			continue
		}
		switch {
		case was && !r.Succeeded:
			regressions = append(regressions, r.TaskID)
		case !was && r.Succeeded:
			fixes = append(fixes, r.TaskID)
		}
	}
	sort.Strings(regressions)
	sort.Strings(fixes)
	return regressions, fixes
}
