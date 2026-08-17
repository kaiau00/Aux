package evalsuite

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Run is the measured outcome of one task.
type Run struct {
	TaskID    string `json:"taskId"`
	Succeeded bool   `json:"succeeded"`

	// FailureReason says how it failed: setup, the agent, or which success
	// command. Without it a red suite is a wall of booleans nobody can act on.
	FailureReason string `json:"failureReason,omitempty"`

	InputTokens  int64 `json:"inputTokens"`
	OutputTokens int64 `json:"outputTokens"`
	// Turns counts model calls, the unit that maps to round trips.
	Turns int64 `json:"turns"`

	Cost float64 `json:"cost"`
	// CostUnknown marks a run whose pricing could not be resolved, so Cost is a
	// lower bound. It propagates to the summary rather than being averaged
	// away, because an unmeasurable cost must never read as a cheap one.
	CostUnknown bool `json:"costUnknown,omitempty"`

	DurationMS int64 `json:"durationMs"`
}

// TotalTokens is what the gate budgets against.
func (r Run) TotalTokens() int64 { return r.InputTokens + r.OutputTokens }

// SuiteRun is one execution of a whole suite.
type SuiteRun struct {
	Suite string    `json:"suite"`
	Label string    `json:"label,omitempty"`
	RanAt time.Time `json:"ranAt"`
	Runs  []Run     `json:"runs"`
}

// Summary is the aggregate a gate compares.
type Summary struct {
	Total       int     `json:"total"`
	Succeeded   int     `json:"succeeded"`
	SuccessRate float64 `json:"successRate"`

	TotalTokens int64 `json:"totalTokens"`
	TotalTurns  int64 `json:"totalTurns"`

	TotalCost   float64 `json:"totalCost"`
	CostUnknown bool    `json:"costUnknown,omitempty"`

	DurationMS int64 `json:"durationMs"`
}

// Summarize aggregates a run.
func (sr SuiteRun) Summarize() Summary {
	s := Summary{Total: len(sr.Runs)}
	for _, r := range sr.Runs {
		if r.Succeeded {
			s.Succeeded++
		}
		s.TotalTokens += r.TotalTokens()
		s.TotalTurns += r.Turns
		s.TotalCost += r.Cost
		s.DurationMS += r.DurationMS
		if r.CostUnknown {
			s.CostUnknown = true
		}
	}
	if s.Total > 0 {
		s.SuccessRate = float64(s.Succeeded) / float64(s.Total)
	}
	return s
}

// Save writes a run so it can serve as a baseline later.
func (sr SuiteRun) Save(path string) error {
	data, err := json.MarshalIndent(sr, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode run: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("failed to write run: %w", err)
	}
	return nil
}

// LoadRun reads a previously saved run.
func LoadRun(path string) (SuiteRun, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SuiteRun{}, fmt.Errorf("failed to read run: %w", err)
	}
	var sr SuiteRun
	if err := json.Unmarshal(data, &sr); err != nil {
		return SuiteRun{}, fmt.Errorf("failed to parse run %s: %w", path, err)
	}
	return sr, nil
}
