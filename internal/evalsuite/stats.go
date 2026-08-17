package evalsuite

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
)

// Series is several runs of the same suite under the same configuration.
//
// Single runs of an agent are not reproducible: on this repository two runs of
// an identical configuration differed by 49% in tokens and by one task in
// success rate. Any comparison built on one run per side is therefore reporting
// noise with a confident face on it, which is worse than reporting nothing.
// Every conclusion has to come from a series.
type Series struct {
	Label   string     `json:"label"`
	Harness string     `json:"harness,omitempty"`
	Runs    []SuiteRun `json:"runs"`
}

// Stat summarizes one measurement across a series.
type Stat struct {
	Median float64 `json:"median"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	N      int     `json:"n"`
}

// Spread is the observed range as a fraction of the median: the noise floor
// this configuration exhibits. An effect smaller than this is not an effect.
func (s Stat) Spread() float64 {
	if s.Median == 0 {
		return 0
	}
	return (s.Max - s.Min) / s.Median
}

// SeriesStats is what a series supports concluding.
type SeriesStats struct {
	Label       string `json:"label"`
	Harness     string `json:"harness,omitempty"`
	Runs        int    `json:"runs"`
	Tokens      Stat   `json:"tokens"`
	Turns       Stat   `json:"turns"`
	SuccessRate Stat   `json:"successRate"`
}

// significanceLevel is the threshold a comparison must clear. Three runs a side
// cannot reach it two-sided (their floor is 0.10), which is deliberate: it means
// the tool reports "more runs needed" rather than certifying a difference off a
// sample too small to establish one.
const significanceLevel = 0.05

// tokenTotals is one number per run: the token total of that whole suite run.
func (s Series) tokenTotals() []float64 {
	out := make([]float64, 0, len(s.Runs))
	for _, r := range s.Runs {
		out = append(out, float64(r.Summarize().TotalTokens))
	}
	return out
}

// mannWhitneyP returns the exact two-sided probability that these two sets of
// runs came from the same distribution, by enumerating every way the combined
// values could have been split between the two groups.
//
// Two-sided because the question asked is "do these differ", not "is B better":
// there was no directional prediction before the runs, and testing one side
// after seeing which way the data fell would manufacture significance.
//
// A consequence worth knowing: with three runs a side the smallest attainable
// two-sided p is 2/20 = 0.10, so n=3 can never reach the 0.05 threshold no
// matter how cleanly the groups separate. Four a side reaches 0.029, five
// reaches 0.008.
//
// Exact enumeration rather than a normal approximation because the sample sizes
// here are tiny, which is exactly where the approximation is worst.
func mannWhitneyP(a, b []float64) float64 {
	na, nb := len(a), len(b)
	if na == 0 || nb == 0 {
		return 1
	}
	// Enumeration is C(na+nb, na); refuse rather than hang on large inputs.
	if na+nb > 24 {
		return 1
	}

	combined := append(append([]float64(nil), a...), b...)
	observed := uStatistic(combined[:na], combined[na:])
	// Under the null the statistic centres on half of all pairs.
	center := float64(na*nb) / 2
	observedDeviation := math.Abs(observed - center)

	var total, atLeastAsExtreme int
	var walk func(start int, chosen []int)
	walk = func(start int, chosen []int) {
		if len(chosen) == na {
			group, rest := split(combined, chosen)
			total++
			if math.Abs(uStatistic(group, rest)-center) >= observedDeviation {
				atLeastAsExtreme++
			}
			return
		}
		for i := start; i < len(combined); i++ {
			walk(i+1, append(chosen, i))
		}
	}
	walk(0, nil)

	if total == 0 {
		return 1
	}
	return float64(atLeastAsExtreme) / float64(total)
}

// uStatistic counts how often a value from x is below a value from y. Ties
// count as half, so equal runs neither support nor oppose a difference.
func uStatistic(x, y []float64) float64 {
	var u float64
	for _, xi := range x {
		for _, yi := range y {
			switch {
			case xi < yi:
				u++
			case xi == yi:
				u += 0.5
			}
		}
	}
	return u
}

func split(values []float64, idx []int) (group, rest []float64) {
	in := make(map[int]bool, len(idx))
	for _, i := range idx {
		in[i] = true
	}
	for i, v := range values {
		if in[i] {
			group = append(group, v)
		} else {
			rest = append(rest, v)
		}
	}
	return group, rest
}

func stat(values []float64) Stat {
	if len(values) == 0 {
		return Stat{}
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	// Median rather than mean: one pathological run (an agent that gives up
	// after two turns, or one that loops) should not drag the summary.
	var median float64
	if n := len(sorted); n%2 == 1 {
		median = sorted[n/2]
	} else {
		median = (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return Stat{Median: median, Min: sorted[0], Max: sorted[len(sorted)-1], N: len(sorted)}
}

// Stats summarizes a series.
func (s Series) Stats() SeriesStats {
	var tokens, turns, rates []float64
	for _, r := range s.Runs {
		sum := r.Summarize()
		tokens = append(tokens, float64(sum.TotalTokens))
		turns = append(turns, float64(sum.TotalTurns))
		rates = append(rates, sum.SuccessRate)
	}
	return SeriesStats{
		Label:       s.Label,
		Harness:     s.Harness,
		Runs:        len(s.Runs),
		Tokens:      stat(tokens),
		Turns:       stat(turns),
		SuccessRate: stat(rates),
	}
}

// SeriesComparison is a comparison of two series, reported against the noise
// each side actually exhibits.
type SeriesComparison struct {
	A SeriesStats `json:"a"`
	B SeriesStats `json:"b"`

	// TokenDelta is B's median relative to A's, as a fraction.
	TokenDelta float64 `json:"tokenDelta"`
	// NoiseFloor is the larger of the two spreads: the smallest difference that
	// could possibly be distinguished from run-to-run variation.
	NoiseFloor float64 `json:"noiseFloor"`
	// P is the exact one-sided rank-test probability that the two sets of runs
	// came from the same distribution. With three runs a side the smallest
	// attainable value is 0.05, so a result at exactly 0.05 is the best that
	// sample size can produce rather than a strong finding.
	P float64 `json:"p"`
	// Conclusive reports whether the difference survives the rank test.
	Conclusive bool `json:"conclusive"`
	// Separated reports whether the two sets of runs overlap at all. Complete
	// separation with a non-significant p means the sample is too small, which
	// is a different problem from the runs genuinely intermingling.
	Separated bool `json:"separated"`
	// Verdict is the sentence a reader should take away.
	Verdict string `json:"verdict"`
}

// CompareSeries compares two configurations honestly.
//
// The rule that matters: a difference smaller than the noise floor is reported
// as inconclusive, never as a result. Two runs of an identical configuration
// can differ by half, so an unqualified "26% cheaper" from single runs is a
// coin flip wearing a number.
func CompareSeries(a, b Series) SeriesComparison {
	sa, sb := a.Stats(), b.Stats()
	c := SeriesComparison{A: sa, B: sb}

	if sa.Tokens.Median > 0 {
		c.TokenDelta = (sb.Tokens.Median - sa.Tokens.Median) / sa.Tokens.Median
	}
	c.NoiseFloor = sa.Tokens.Spread()
	if sb.Tokens.Spread() > c.NoiseFloor {
		c.NoiseFloor = sb.Tokens.Spread()
	}

	magnitude := c.TokenDelta
	if magnitude < 0 {
		magnitude = -magnitude
	}

	// Significance is decided by a rank test, not by comparing the median gap
	// to the spread.
	//
	// The spread heuristic this replaced was too crude: it conflates variation
	// *within* each configuration with separation *between* them. On the first
	// real comparison it called a result inconclusive (19% gap, 29% spread)
	// when in fact every run of one configuration used fewer tokens than every
	// run of the other — which is the strongest separation three samples can
	// show. Overlap is the question, and only a rank test answers it.
	c.P = mannWhitneyP(a.tokenTotals(), b.tokenTotals())
	c.Conclusive = c.P <= significanceLevel
	c.Separated = disjoint(a.tokenTotals(), b.tokenTotals())

	switch {
	case sa.Runs < 2 || sb.Runs < 2:
		c.Conclusive = false
		c.Verdict = "Inconclusive: at least two runs per configuration are needed to see run-to-run variation at all."
	case !c.Conclusive && c.Separated:
		// Separated but underpowered is a different situation from overlapping,
		// and conflating them would tell someone to collect more data when the
		// data already disagrees with them, or the reverse.
		c.Verdict = fmt.Sprintf(
			"Suggestive but not established: every %s run used %s tokens than every %s run (%.0f%% median difference), "+
				"but %d runs a side cannot reach p<=%.2f two-sided — the floor at this sample size is p=%.2f. Need %d+ runs a side.",
			lowerLabel(c), directionWord(c), higherLabel(c), magnitude*100,
			sa.Runs, significanceLevel, c.P, neededRuns(sa.Runs))
	case !c.Conclusive:
		c.Verdict = fmt.Sprintf(
			"Inconclusive: %.0f%% difference in median tokens, but the runs overlap (p=%.2f). More runs would be needed to tell this from variation.",
			magnitude*100, c.P)
	case c.TokenDelta < 0:
		c.Verdict = fmt.Sprintf(
			"%s used %.0f%% fewer median tokens than %s (p=%.2f). Suggestive at this sample size, not established.",
			sb.Label, magnitude*100, sa.Label, c.P)
	default:
		c.Verdict = fmt.Sprintf(
			"%s used %.0f%% more median tokens than %s (p=%.2f). Suggestive at this sample size, not established.",
			sb.Label, magnitude*100, sa.Label, c.P)
	}

	// Capability still governs. A cheaper configuration that solves less is not
	// a better one, and this must not be buried under a token headline.
	if sb.SuccessRate.Median < sa.SuccessRate.Median {
		c.Conclusive = false
		c.Verdict = fmt.Sprintf(
			"%s solved less (median %.0f%% vs %.0f%%). Token differences are moot until that is explained.",
			sb.Label, sb.SuccessRate.Median*100, sa.SuccessRate.Median*100)
	}
	return c
}

// Save writes a series.
func (s Series) Save(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode series: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("failed to write series: %w", err)
	}
	return nil
}

// LoadSeries reads a series, accepting a single run for convenience.
func LoadSeries(path string) (Series, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Series{}, fmt.Errorf("failed to read series: %w", err)
	}
	var s Series
	if json.Unmarshal(data, &s) == nil && len(s.Runs) > 0 {
		return s, nil
	}
	// Fall back to a single SuiteRun file.
	var one SuiteRun
	if err := json.Unmarshal(data, &one); err != nil {
		return Series{}, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return Series{Label: one.Label, Harness: one.Harness, Runs: []SuiteRun{one}}, nil
}

// disjoint reports whether every value in one group beats every value in the
// other, i.e. the two sets of runs do not overlap.
func disjoint(a, b []float64) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	maxA, minA := a[0], a[0]
	for _, v := range a {
		maxA, minA = math.Max(maxA, v), math.Min(minA, v)
	}
	maxB, minB := b[0], b[0]
	for _, v := range b {
		maxB, minB = math.Max(maxB, v), math.Min(minB, v)
	}
	return maxA < minB || maxB < minA
}

// neededRuns is the smallest equal group size whose two-sided floor clears the
// significance level, so the verdict can say what would actually settle it.
func neededRuns(current int) int {
	for n := current + 1; n <= 12; n++ {
		if 2/binomial(2*n, n) <= significanceLevel {
			return n
		}
	}
	return 12
}

func binomial(n, k int) float64 {
	r := 1.0
	for i := 0; i < k; i++ {
		r = r * float64(n-i) / float64(i+1)
	}
	return r
}

func lowerLabel(c SeriesComparison) string {
	if c.TokenDelta < 0 {
		return c.B.Label
	}
	return c.A.Label
}

func higherLabel(c SeriesComparison) string {
	if c.TokenDelta < 0 {
		return c.A.Label
	}
	return c.B.Label
}

func directionWord(SeriesComparison) string { return "fewer" }
