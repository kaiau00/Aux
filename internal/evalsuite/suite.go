// Package evalsuite runs a suite of real coding tasks against Aux and measures
// what they cost, so claims about the harness being cheaper or better can be
// checked instead of asserted.
//
// The distinction from internal/eval: that package compares prompt compilers
// over synthetic fixtures and computes changes-per-dollar from durable records.
// This one runs whole tasks end to end against real repositories, which is the
// only thing that can tell you whether a change helped or quietly broke
// something.
//
// Success is deliberately decided by commands, not by a model. Each task names
// commands that must exit zero afterwards. A model judging its own work is the
// circularity this suite exists to escape, and a test that passes is not an
// opinion.
package evalsuite

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Task is one benchmark task: a repository at a known revision, a prompt, and a
// deterministic way to tell whether the work was actually done.
type Task struct {
	// ID is stable across runs; it is how a task's results are compared over
	// time, so renaming one discards its history.
	ID string `json:"id"`

	// Description is for humans reading a report.
	Description string `json:"description,omitempty"`

	// Repo is the path to a git repository to run the task in. The runner
	// resets it to BaseRevision before every run, so it must be a checkout you
	// are willing to have reset --hard.
	Repo string `json:"repo"`

	// BaseRevision pins the starting state. Without it a task silently measures
	// a different problem as the repository moves.
	BaseRevision string `json:"baseRevision"`

	// Prompt is what the agent is asked to do.
	Prompt string `json:"prompt"`

	// Setup runs before the agent, for anything the task needs in place
	// (installing dependencies, seeding fixtures). Failures abort the task.
	Setup []string `json:"setup,omitempty"`

	// Success commands all have to exit zero for the task to count as passed.
	// These carry the whole weight of the benchmark: a task whose success
	// command is too loose reports progress that did not happen.
	Success []string `json:"success"`

	// Corrected marks tasks where the agent previously had to be corrected by
	// hand. These are the most valuable tasks in any suite — they encode what
	// it actually gets wrong, rather than what it was already good at — so
	// reports call out how many of them a suite contains.
	Corrected bool `json:"corrected,omitempty"`
}

// Suite is a named set of tasks.
type Suite struct {
	Name  string `json:"name"`
	Tasks []Task `json:"tasks"`
}

// Validate reports every problem with a suite at once, rather than one per run,
// because these are usually fixed in a single editing pass.
func (s Suite) Validate() error {
	var problems []string

	if strings.TrimSpace(s.Name) == "" {
		problems = append(problems, "suite has no name")
	}
	if len(s.Tasks) == 0 {
		problems = append(problems, "suite has no tasks")
	}

	seen := make(map[string]bool, len(s.Tasks))
	for i, t := range s.Tasks {
		where := fmt.Sprintf("task %d", i)
		if t.ID != "" {
			where = fmt.Sprintf("task %q", t.ID)
		}
		switch {
		case strings.TrimSpace(t.ID) == "":
			problems = append(problems, where+": missing id")
		case seen[t.ID]:
			problems = append(problems, where+": duplicate id")
		}
		seen[t.ID] = true

		if strings.TrimSpace(t.Repo) == "" {
			problems = append(problems, where+": missing repo")
		}
		if strings.TrimSpace(t.BaseRevision) == "" {
			// Not cosmetic: without a pinned revision the task measures a
			// moving target and results are not comparable across runs.
			problems = append(problems, where+": missing baseRevision, so results would not be comparable between runs")
		}
		if strings.TrimSpace(t.Prompt) == "" {
			problems = append(problems, where+": missing prompt")
		}
		if len(t.Success) == 0 {
			// A task with no success command always "passes", which would
			// inflate the success rate that everything else is gated on.
			problems = append(problems, where+": no success commands, so it could never fail")
		}
	}

	if len(problems) > 0 {
		return fmt.Errorf("invalid suite:\n  - %s", strings.Join(problems, "\n  - "))
	}
	return nil
}

// CorrectedCount reports how many tasks came from real corrections.
func (s Suite) CorrectedCount() int {
	n := 0
	for _, t := range s.Tasks {
		if t.Corrected {
			n++
		}
	}
	return n
}

// LoadSuite reads and validates a suite file.
func LoadSuite(path string) (Suite, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Suite{}, fmt.Errorf("failed to read suite: %w", err)
	}
	var s Suite
	if err := json.Unmarshal(data, &s); err != nil {
		return Suite{}, fmt.Errorf("failed to parse suite %s: %w", path, err)
	}
	if err := s.Validate(); err != nil {
		return Suite{}, err
	}
	return s, nil
}
