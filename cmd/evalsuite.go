package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aux-ai/aux-cli/internal/config"
	"github.com/aux-ai/aux-cli/internal/cost"
	"github.com/aux-ai/aux-cli/internal/db"
	"github.com/aux-ai/aux-cli/internal/evalsuite"
	"github.com/spf13/cobra"
)

// ledgerMetrics adapts the cost ledger to evalsuite's MetricsReader. Metrics
// come from the durable ledger rather than being parsed out of agent output, so
// a run's cost is measured the same way whether it was watched or not.
type ledgerMetrics struct{ ledger cost.Service }

func (m ledgerMetrics) TaskMetrics(ctx context.Context, sessionID string) (int64, int64, int64, float64, bool, error) {
	totals, err := m.ledger.SessionTotals(ctx, sessionID)
	if err != nil {
		return 0, 0, 0, 0, false, err
	}
	return totals.InputTokens, totals.OutputTokens, totals.Calls, totals.Cost, totals.CostUnknown, nil
}

var evalSuiteCmd = &cobra.Command{
	Use:   "suite <suite.json>",
	Short: "Run a task benchmark suite and record what it cost",
	Long: `Run a suite of real coding tasks and measure tokens, turns, and cost.

Each task resets its repository to a pinned revision, runs the agent
non-interactively, and then runs the task's success commands. Those commands
decide whether the task passed — not the agent's exit code, and not a model
judging its own work.

This spends real API budget. Run --validate first to check a suite without
executing anything.

To gate a change, record a baseline, make the change, run again, and compare:

  aux eval suite bench/suite.json --save bench/baseline.json
  aux eval suite bench/suite.json --save bench/candidate.json
  aux eval gate bench/baseline.json bench/candidate.json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		suite, err := evalsuite.LoadSuite(args[0])
		if err != nil {
			return err
		}

		validateOnly, _ := cmd.Flags().GetBool("validate")
		if validateOnly {
			fmt.Printf("%s: %d task(s) valid, %d from real corrections.\n",
				suite.Name, len(suite.Tasks), suite.CorrectedCount())
			if suite.CorrectedCount() == 0 {
				fmt.Println("\nNote: no tasks are marked \"corrected\". A suite built only from" +
					"\ntasks the agent already handles measures what it is good at, not what" +
					"\nit gets wrong.")
			}
			return nil
		}

		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		if _, err := config.Load(cwd, false); err != nil {
			return err
		}
		conn, err := db.Connect()
		if err != nil {
			return err
		}
		defer conn.Close()

		binary, _ := cmd.Flags().GetString("binary")
		label, _ := cmd.Flags().GetString("label")
		timeout, _ := cmd.Flags().GetDuration("timeout")

		runner := evalsuite.NewRunner(
			evalsuite.ShellExecutor{Timeout: timeout},
			ledgerMetrics{ledger: cost.NewService(conn)},
			binary,
		)

		fmt.Printf("Running %s: %d task(s). This spends API budget.\n\n", suite.Name, len(suite.Tasks))
		result, err := runner.RunSuite(cmd.Context(), suite, label)
		if err != nil {
			return err
		}

		fmt.Print(evalsuite.RenderRun(result))

		if path, _ := cmd.Flags().GetString("save"); path != "" {
			if err := result.Save(path); err != nil {
				return err
			}
			fmt.Printf("\nSaved to %s\n", path)
		}
		return nil
	},
}

var evalGateCmd = &cobra.Command{
	Use:   "gate <baseline.json> <candidate.json>",
	Short: "Decide whether a candidate run may ship",
	Long: `Compare a candidate suite run against a baseline.

Success rate is a hard floor: a candidate that uses fewer tokens while solving
fewer tasks fails, because that is not a cheaper harness, it is a worse one.
Token and turn budgets are only considered once capability is known to hold.

Exits non-zero when the gate fails, so it can be used in CI.`,
	Args: cobra.ExactArgs(2),
	// A failing gate is a verdict, not a usage error; printing the help text
	// after it buries the reason the change was rejected.
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		baseline, err := evalsuite.LoadRun(args[0])
		if err != nil {
			return err
		}
		candidate, err := evalsuite.LoadRun(args[1])
		if err != nil {
			return err
		}

		th := evalsuite.DefaultThresholds()
		if v, _ := cmd.Flags().GetFloat64("token-ratio"); v > 0 {
			th.TokenRatio = v
		}
		if v, _ := cmd.Flags().GetFloat64("turn-ratio"); v > 0 {
			th.TurnRatio = v
		}

		verdict := evalsuite.Gate(baseline, candidate, th)
		fmt.Print(evalsuite.RenderVerdict(verdict))
		if !verdict.Passed {
			// Silent failure in CI is the whole thing this is meant to prevent.
			return fmt.Errorf("gate failed")
		}
		return nil
	},
}

func init() {
	evalSuiteCmd.Flags().Bool("validate", false, "Check the suite without running anything")
	evalSuiteCmd.Flags().String("binary", "aux", "Command used to invoke the agent")
	evalSuiteCmd.Flags().String("label", "", "Label recorded with the run (e.g. \"paging-on\")")
	evalSuiteCmd.Flags().String("save", "", "Write the run to this path for later comparison")
	evalSuiteCmd.Flags().Duration("timeout", 10*time.Minute, "Per-command timeout")

	evalGateCmd.Flags().Float64("token-ratio", 0, "Token budget as a fraction of baseline (default 0.7)")
	evalGateCmd.Flags().Float64("turn-ratio", 0, "Turn ceiling as a multiple of baseline (default 1.1)")

	evalCmd.AddCommand(evalSuiteCmd)
	evalCmd.AddCommand(evalGateCmd)
}
