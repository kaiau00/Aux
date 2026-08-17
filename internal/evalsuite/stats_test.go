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

// The finding this whole mechanism exists for: two runs of one configuration
// differed by 49%, so a 26% difference between configurations proves nothing.
func TestDifferenceInsideTheNoiseFloorIsInconclusive(t *testing.T) {
	a := seriesOf("baseline", 660000, 334000, 500000) // ~65% spread
	b := seriesOf("paging", 484000, 400000, 450000)

	c := CompareSeries(a, b)
	if c.Conclusive {
		t.Fatalf("a difference smaller than the noise must not be conclusive: %+v", c)
	}
	if !strings.Contains(c.Verdict, "noise floor") {
		t.Fatalf("the verdict should name the noise floor, got %q", c.Verdict)
	}
}

func TestDifferenceExceedingTheNoiseFloorIsConclusive(t *testing.T) {
	a := seriesOf("aux", 100000, 102000, 101000) // tight
	b := seriesOf("opencode", 40000, 41000, 40500)

	c := CompareSeries(a, b)
	if !c.Conclusive {
		t.Fatalf("a 60%% difference against ~2%% noise should be conclusive: %+v", c)
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
