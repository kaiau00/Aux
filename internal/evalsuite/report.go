package evalsuite

import (
	"fmt"
	"strings"
)

// RenderRun formats a suite run for a terminal.
func RenderRun(sr SuiteRun) string {
	var b strings.Builder
	s := sr.Summarize()

	fmt.Fprintf(&b, "%s", sr.Suite)
	if sr.Label != "" {
		fmt.Fprintf(&b, " (%s)", sr.Label)
	}
	b.WriteString("\n")
	b.WriteString(strings.Repeat("-", 60) + "\n")

	for _, r := range sr.Runs {
		mark := "FAIL"
		if r.Succeeded {
			mark = "pass"
		}
		fmt.Fprintf(&b, "%-4s  %-28s %7d tok  %3d turns  %6.1fs\n",
			mark, truncate(r.TaskID, 28), r.TotalTokens(), r.Turns,
			float64(r.DurationMS)/1000)
		if r.FailureReason != "" {
			fmt.Fprintf(&b, "      %s\n", truncate(oneLine(r.FailureReason), 200))
		}
	}

	b.WriteString(strings.Repeat("-", 60) + "\n")
	fmt.Fprintf(&b, "%d/%d passed (%.0f%%)   %d tokens   %d turns",
		s.Succeeded, s.Total, s.SuccessRate*100, s.TotalTokens, s.TotalTurns)
	if s.CostUnknown {
		// Never present a partially-measured cost as if it were the real one.
		fmt.Fprintf(&b, "   cost >= $%.2f (incomplete)\n", s.TotalCost)
	} else {
		fmt.Fprintf(&b, "   $%.2f\n", s.TotalCost)
	}
	return b.String()
}

// RenderVerdict formats a gate decision, leading with the answer.
func RenderVerdict(v Verdict) string {
	var b strings.Builder

	if v.Passed {
		b.WriteString("GATE PASSED\n\n")
	} else {
		b.WriteString("GATE FAILED\n\n")
	}

	for _, f := range v.Failures {
		fmt.Fprintf(&b, "  x %s\n", f)
	}
	for _, n := range v.Notes {
		fmt.Fprintf(&b, "  - %s\n", n)
	}
	if len(v.Failures) > 0 || len(v.Notes) > 0 {
		b.WriteString("\n")
	}

	if len(v.Fixes) > 0 {
		fmt.Fprintf(&b, "  Now passing: %s\n\n", strings.Join(v.Fixes, ", "))
	}

	fmt.Fprintf(&b, "%-12s %10s %10s\n", "", "baseline", "candidate")
	fmt.Fprintf(&b, "%-12s %9.0f%% %9.0f%%\n", "success", v.Baseline.SuccessRate*100, v.Candidate.SuccessRate*100)
	fmt.Fprintf(&b, "%-12s %10d %10d\n", "tokens", v.Baseline.TotalTokens, v.Candidate.TotalTokens)
	fmt.Fprintf(&b, "%-12s %10d %10d\n", "turns", v.Baseline.TotalTurns, v.Candidate.TotalTurns)

	return b.String()
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
