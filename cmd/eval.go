package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/aux-ai/aux-cli/internal/config"
	"github.com/aux-ai/aux-cli/internal/db"
	"github.com/aux-ai/aux-cli/internal/eval"
	"github.com/aux-ai/aux-cli/internal/eventstore"
	"github.com/spf13/cobra"
)

// evalCmd runs the deterministic, network-free evaluation comparing the
// compatibility and paging prompt compilers over baseline fixtures
// (roadmapplan.md §19 PR 12).
var evalCmd = &cobra.Command{
	Use:   "eval",
	Short: "Run the local prompt-compiler evaluation (control vs optimized)",
}

var evalCompilerCmd = &cobra.Command{
	Use:   "compiler",
	Short: "Compare compatibility vs paging prompt compilation on baseline fixtures",
	RunE: func(cmd *cobra.Command, _ []string) error {
		fmt.Print(eval.RenderReport(eval.RunBaseline()))
		return nil
	},
}

var evalExperimentCmd = &cobra.Command{
	Use:   "experiment",
	Short: "Run and persist the compiler experiment (compatibility vs paging)",
	RunE: func(cmd *cobra.Command, _ []string) error {
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

		exp, results, err := eval.RunCompilerExperiment(context.Background(), eval.NewExperimentStore(conn), "")
		if err != nil {
			return err
		}
		fmt.Printf("Experiment %s persisted (%d runs).\n\n", exp.ID, len(results))
		fmt.Print(eval.RenderReport(results))
		return nil
	},
}

var evalReplayCmd = &cobra.Command{
	Use:   "replay <task-id>",
	Short: "Deterministically replay a task's state from its durable events",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
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

		ctx := context.Background()
		events, err := eventstore.NewService(conn).List(ctx, eventstore.Filter{TaskID: args[0]})
		if err != nil {
			return err
		}
		rt := eval.ReplayTaskState(args[0], events)
		fmt.Printf("Task %s replayed from %d events:\n", rt.TaskID, rt.Events)
		fmt.Printf("  status=%s turns=%d modelCalls=%d toolCalls=%d failures=%d validated=%v\n",
			rt.Status, rt.Turns, rt.ModelCalls, rt.ToolCalls, rt.Failures, rt.Validated)
		return nil
	},
}

func init() {
	evalCmd.AddCommand(evalCompilerCmd)
	evalCmd.AddCommand(evalExperimentCmd)
	evalCmd.AddCommand(evalReplayCmd)
	rootCmd.AddCommand(evalCmd)
}
