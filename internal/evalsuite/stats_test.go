package evalsuite

import (
	"strings"
	"testing"
)

func seriesOf(label string, totals ...int64) Series {
	s := Series{Label: label}
	for _, tot := range totals {
		s.Runs = append(s.Runs, SuiteRun{Runs: []Run{{TaskID: "a", Succeeded: true, InputTokens: tot, Turns: 5}}})
	}
	return s
}

// Overlapping runs are inconclusive however far apart the medians look: if a
// value from one configuration outranks a value from the other, the difference
// is not separable from run-to-run variation at this sample size.
func TestOverlappingRunsAreInconclusive(t *testing.T) {
	a := seriesOf("baseline", 660000, 334000, 500000)
	b := seriesOf("paging", 484000, 400000, 450000)

	c := CompareSeries(a, b)
	if c.Conclusive {
		t.Fatalf("overlapping runs must not be conclusive: %+v", c)
	}
	if !strings.Contains(c.Verdict, "overlap") {
		t.Fatalf("the verdict should say the runs overlap, got %q", c.Verdict)
	}
}

// The case the first real comparison exposed: a spread-based heuristic called
// this inconclusive (19% gap against a 29% spread) even though every run of one
// side beat every run of the other, which is the most three samples can show.
// The real aux-vs-opencode numbers. Every aux run beat every opencode run --
// the most three samples can show -- and it still cannot clear p<=0.05
// two-sided, whose floor at n=3 is 0.10. The tool must say "too few runs"
// rather than either certifying or dismissing it.
func TestSeparatedButUnderpoweredIsDistinguishedFromOverlap(t *testing.T) {
	aux := seriesOf("aux", 430576, 587790, 591400)
	oc := seriesOf("opencode", 698535, 699974, 901503)

	c := CompareSeries(aux, oc)
	if !c.Separated {
		t.Fatal("these runs do not overlap and must be reported as separated")
	}
	if c.Conclusive {
		t.Fatalf("n=3 cannot establish significance two-sided, got p=%.3f", c.P)
	}
	if c.P < 0.09 || c.P > 0.11 {
		t.Fatalf("perfect separation at n=3 should give p=0.10, got %.3f", c.P)
	}
	if strings.Contains(c.Verdict, "overlap") {
		t.Fatalf("these runs do not overlap; the verdict must not say they do: %q", c.Verdict)
	}
	for _, want := range []string{"Suggestive", "not established", "runs a side"} {
		if !strings.Contains(c.Verdict, want) {
			t.Errorf("verdict should contain %q, got %q", want, c.Verdict)
		}
	}
}

// Identical data cannot be evidence of a difference.
func TestIdenticalSeriesAreNeverConclusive(t *testing.T) {
	s := seriesOf("x", 100000, 110000, 105000)
	if c := CompareSeries(s, s); c.Conclusive {
		t.Fatalf("a series compared with itself must not be conclusive, p=%.3f", c.P)
	}
}

func TestDifferenceExceedingTheNoiseFloorIsConclusive(t *testing.T) {
	// Five a side: the two-sided floor is 0.008, so separation can be
	// established. At three a side the same data could not be.
	a := seriesOf("aux", 100000, 102000, 101000, 103000, 99000)
	b := seriesOf("opencode", 40000, 41000, 40500, 39000, 42000)

	c := CompareSeries(a, b)
	if !c.Conclusive {
		t.Fatalf("cleanly separated runs should be conclusive: %+v", c)
	}
	if !strings.Contains(c.Verdict, "fewer") {
		t.Fatalf("the verdict should state the direction, got %q", c.Verdict)
	}
}

// One run per side cannot show a noise floor at all, so it can never conclude.
func TestSingleRunSeriesAreNeverConclusive(t *testing.T) {
	c := CompareSeries(seriesOf("a", 100000), seriesOf("b", 10000))
	if c.Conclusive {
		t.Fatal("one run per side must never yield a conclusion")
	}
	if !strings.Contains(c.Verdict, "at least two runs") {
		t.Fatalf("the verdict should say why, got %q", c.Verdict)
	}
}

// Capability governs: a cheaper configuration that solves less is not better,
// and that must not be buried under a token headline.
func TestLowerSuccessRateOverridesATokenWin(t *testing.T) {
	a := Series{Label: "baseline", Runs: []SuiteRun{
		{Runs: []Run{{TaskID: "x", Succeeded: true, InputTokens: 100000}}},
		{Runs: []Run{{TaskID: "x", Succeeded: true, InputTokens: 100000}}},
	}}
	b := Series{Label: "cheap", Runs: []SuiteRun{
		{Runs: []Run{{TaskID: "x", Succeeded: false, InputTokens: 10000}}},
		{Runs: []Run{{TaskID: "x", Succeeded: false, InputTokens: 10000}}},
	}}

	c := CompareSeries(a, b)
	if c.Conclusive {
		t.Fatal("a token win must not be conclusive when the success rate fell")
	}
	if !strings.Contains(c.Verdict, "solved less") {
		t.Fatalf("the verdict should lead with the capability loss, got %q", c.Verdict)
	}
}

func TestMedianResistsOnePathologicalRun(t *testing.T) {
	// Four normal runs and one that gave up early.
	s := seriesOf("x", 100000, 105000, 98000, 102000, 5000)
	st := s.Stats()
	if st.Tokens.Median < 90000 {
		t.Fatalf("median should resist the outlier, got %.0f", st.Tokens.Median)
	}
	if st.Tokens.Spread() < 0.5 {
		t.Fatal("the spread must still expose that the outlier happened")
	}
}

func TestSpreadIsZeroForIdenticalRuns(t *testing.T) {
	if got := seriesOf("x", 1000, 1000, 1000).Stats().Tokens.Spread(); got != 0 {
		t.Fatalf("identical runs should show no spread, got %v", got)
	}
}
