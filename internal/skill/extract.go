package skill

import (
	"fmt"
	"sort"
	"strings"
)

// ExtractInput is the deterministic evidence available once a task completes.
// It intentionally mirrors memory.ExtractInput: both answer "what did this task
// prove?", memory for facts and skills for repeatable procedure.
type ExtractInput struct {
	ProjectID string
	TaskID    string
	Objective string
	// SuccessfulCommands are validation commands that actually passed during
	// the task. They are the only evidence used, because they are the only part
	// of a task that is both reusable and known to work.
	SuccessfulCommands []string
}

// Extract proposes skill candidates from a completed task.
//
// It is deliberately narrow. A skill that is wrong is worse than no skill,
// because it fires on future tasks and misleads them, so nothing here is
// inferred or summarized — a command either passed validation during this task
// or it does not appear. Anything richer belongs behind the eval gate, not in a
// deterministic extractor.
//
// The result is a candidate. Candidates are inert: they are never assembled into
// a prompt until an evaluation passes and someone promotes them, so proposing
// one costs the user nothing and interrupts nobody.
func Extract(in ExtractInput) []Content {
	commands := dedupeNonEmpty(in.SuccessfulCommands)
	if in.ProjectID == "" || len(commands) == 0 {
		return nil
	}

	steps := make([]Step, 0, len(commands))
	for _, cmd := range commands {
		steps = append(steps, Step{
			Title:  "Run " + cmd,
			Action: cmd,
		})
	}

	return []Content{{
		Name:    "validate-project",
		Purpose: "Check that changes to this project are sound, using the commands that have actually passed here.",
		Scope:   "project",
		Triggers: []string{
			"about to report work as done",
			"asked whether a change is safe",
		},
		// A validated command proves the command works, not that it is
		// sufficient. Saying so keeps a future task from treating a green run as
		// a broader guarantee than it is.
		Exclusions: []string{
			"do not treat a passing run as proof that untested behaviour is correct",
		},
		Procedure:              steps,
		ToolRequirements:       []string{"bash"},
		ValidationRequirements: commands,
		Outputs:                []string{"pass/fail per command, with output"},
	}}
}

// SourceIDsFor returns the provenance ids for a task-derived candidate.
func SourceIDsFor(in ExtractInput) []string {
	if in.TaskID == "" {
		return nil
	}
	return []string{in.TaskID}
}

// Describe renders a short human-readable summary of a candidate, for CLI and
// dashboard listings.
func Describe(c Content) string {
	return fmt.Sprintf("%s (%d step%s)", c.Name, len(c.Procedure), plural(len(c.Procedure)))
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// dedupeNonEmpty removes blanks and duplicates and sorts, so the same set of
// commands discovered in a different order does not produce a spuriously
// different candidate version.
func dedupeNonEmpty(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
