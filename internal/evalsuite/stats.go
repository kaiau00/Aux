package evalsuite

import (
	"encoding/json"
	"fmt"
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
	// Conclusive reports whether the observed difference exceeds the noise.
	Conclusive bool `json:"conclusive"`
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
	c.Conclusive = magnitude > c.NoiseFloor

	switch {
	case sa.Runs < 2 || sb.Runs < 2:
		c.Conclusive = false
		c.Verdict = "Inconclusive: at least two runs per configuration are needed to see the noise floor at all."
	case !c.Conclusive:
		c.Verdict = fmt.Sprintf(
			"Inconclusive: the %.0f%% difference in median tokens is within the %.0f%% run-to-run noise floor.",
			magnitude*100, c.NoiseFloor*100)
	case c.TokenDelta < 0:
		c.Verdict = fmt.Sprintf(
			"%s used %.0f%% fewer median tokens than %s, exceeding the %.0f%% noise floor.",
			sb.Label, magnitude*100, sa.Label, c.NoiseFloor*100)
	default:
		c.Verdict = fmt.Sprintf(
			"%s used %.0f%% more median tokens than %s, exceeding the %.0f%% noise floor.",
			sb.Label, magnitude*100, sa.Label, c.NoiseFloor*100)
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
