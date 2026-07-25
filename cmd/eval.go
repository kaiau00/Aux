package cmd

import (
	"fmt"

	"github.com/aux-ai/aux-cli/internal/eval"
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

func init() {
	evalCmd.AddCommand(evalCompilerCmd)
	rootCmd.AddCommand(evalCmd)
}
