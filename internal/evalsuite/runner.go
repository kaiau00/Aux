package evalsuite

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Executor runs a shell command in a directory and reports whether it
// succeeded. It is injected so the runner's control flow — reset, setup, agent,
// success checks, metric capture — can be tested without a provider, a network,
// or a real repository.
type Executor interface {
	Run(ctx context.Context, dir string, command string) (stdout string, err error)
}

// MetricsReader supplies what a task run cost, read from the durable ledger
// after the agent finishes rather than parsed out of its output.
type MetricsReader interface {
	TaskMetrics(ctx context.Context, sessionID string) (inputTokens, outputTokens, turns int64, cost float64, costUnknown bool, err error)
}

// ShellExecutor runs commands with the system shell.
type ShellExecutor struct {
	// Timeout bounds a single command. A benchmark that hangs on one task is
	// worse than one that fails it, because nobody watches a suite run.
	Timeout time.Duration
}

func (e ShellExecutor) Run(ctx context.Context, dir, command string) (string, error) {
	timeout := e.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return string(out), fmt.Errorf("command timed out after %s: %s", timeout, command)
	}
	return string(out), err
}

// Runner executes a suite.
type Runner struct {
	Exec    Executor
	Metrics MetricsReader

	// AuxBinary is the command used to invoke the agent non-interactively.
	AuxBinary string

	// Isolate, when set, resets each repository to the task's base revision
	// before running. Defaults on: without it, task N runs against the mess
	// task N-1 left, and the suite measures order rather than capability.
	//
	// It runs `git reset --hard` and `git clean -fd`, which is destructive, so
	// suite repositories must be scratch checkouts.
	Isolate bool
}

// NewRunner returns a runner with isolation enabled.
func NewRunner(exec Executor, metrics MetricsReader, auxBinary string) *Runner {
	return &Runner{Exec: exec, Metrics: metrics, AuxBinary: auxBinary, Isolate: true}
}

// RunSuite executes every task and returns the measured result.
//
// A task that errors is recorded as failed rather than aborting the suite: a
// partial result with a named failure is more useful than no result, and one
// broken task should not cost the whole run.
func (r *Runner) RunSuite(ctx context.Context, s Suite, label string) (SuiteRun, error) {
	if err := s.Validate(); err != nil {
		return SuiteRun{}, err
	}
	if r.Exec == nil {
		return SuiteRun{}, fmt.Errorf("runner has no executor")
	}

	out := SuiteRun{Suite: s.Name, Label: label, RanAt: time.Now()}
	for _, t := range s.Tasks {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		out.Runs = append(out.Runs, r.runTask(ctx, t))
	}
	return out, nil
}

func (r *Runner) runTask(ctx context.Context, t Task) Run {
	started := time.Now()
	run := Run{TaskID: t.ID}
	finish := func(reason string) Run {
		run.FailureReason = reason
		run.DurationMS = time.Since(started).Milliseconds()
		return run
	}

	if r.Isolate {
		for _, cmd := range []string{
			"git reset --hard " + t.BaseRevision,
			"git clean -fd",
		} {
			if out, err := r.Exec.Run(ctx, t.Repo, cmd); err != nil {
				return finish(fmt.Sprintf("isolation failed (%s): %v: %s", cmd, err, out))
			}
		}
	}

	for _, cmd := range t.Setup {
		if out, err := r.Exec.Run(ctx, t.Repo, cmd); err != nil {
			return finish(fmt.Sprintf("setup failed (%s): %v: %s", cmd, err, out))
		}
	}

	// The agent's own exit status is not the measurement — a task can exit
	// cleanly having done nothing, or exit non-zero having done the work. Only
	// the success commands decide. A hard failure is still recorded, because it
	// explains an otherwise mysterious red.
	//
	// JSON output is requested for the session id, which is the handle used to
	// read what the run cost from the ledger.
	agentCmd := fmt.Sprintf("%s -p %s --quiet --output-format json", r.AuxBinary, shellQuote(t.Prompt))
	agentOut, agentErr := r.Exec.Run(ctx, t.Repo, agentCmd)

	sessionID, idErr := sessionIDFrom(agentOut)
	if r.Metrics != nil {
		if idErr != nil {
			// Unmeasured cost must be visible, not silently zero: a run with no
			// metrics would otherwise look like the cheapest in the suite.
			run.CostUnknown = true
		} else if in, outTok, turns, cost, unknown, err := r.Metrics.TaskMetrics(ctx, sessionID); err == nil {
			run.InputTokens, run.OutputTokens, run.Turns = in, outTok, turns
			run.Cost, run.CostUnknown = cost, unknown
		} else {
			run.CostUnknown = true
		}
	}

	for _, cmd := range t.Success {
		out, err := r.Exec.Run(ctx, t.Repo, cmd)
		if err != nil {
			reason := fmt.Sprintf("success command failed (%s): %v: %s", cmd, err, truncate(out, 2000))
			if agentErr != nil {
				reason += fmt.Sprintf(" [agent also exited with error: %v]", agentErr)
			}
			return finish(reason)
		}
	}

	run.Succeeded = true
	run.DurationMS = time.Since(started).Milliseconds()
	return run
}

// shellQuote wraps a string in single quotes for `sh -c`. Prompts are arbitrary
// text from a suite file and routinely contain quotes and newlines.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// sessionIDFrom pulls the session id out of `aux -p --output-format json`.
//
// The agent's stdout may carry other lines ahead of the JSON object, so this
// scans for the last well-formed object carrying a session id rather than
// assuming the whole of stdout parses.
func sessionIDFrom(out string) (string, error) {
	start := strings.LastIndex(out, "{")
	for start >= 0 {
		var payload struct {
			SessionID string `json:"sessionId"`
		}
		if err := json.Unmarshal([]byte(out[start:]), &payload); err == nil && payload.SessionID != "" {
			return payload.SessionID, nil
		}
		start = strings.LastIndex(out[:start], "{")
	}
	return "", fmt.Errorf("no session id in agent output")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "... [truncated]"
}
