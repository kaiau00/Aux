package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aux-ai/aux-cli/internal/config"
	"github.com/aux-ai/aux-cli/internal/db"
	"github.com/aux-ai/aux-cli/internal/task"
	"github.com/spf13/cobra"
)

// taskCmd is a read-only surface over first-class tasks (roadmapplan.md §6.7).
var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Inspect first-class tasks and their specs",
}

var taskShowCmd = &cobra.Command{
	Use:   "show <task-id>",
	Short: "Show a task, its compiled spec, and acceptance criteria",
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
		store := task.NewStore(conn)
		tk, err := store.GetTask(ctx, args[0])
		if err != nil {
			return fmt.Errorf("task not found: %w", err)
		}
		spec, _, err := store.LatestSpec(ctx, tk.ID)
		if err != nil {
			return err
		}

		if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(map[string]any{"task": tk, "spec": spec})
		}

		fmt.Printf("Task:      %s\n", tk.ID)
		fmt.Printf("Objective: %s\n", tk.Objective)
		fmt.Printf("Mode:      %s\n", tk.Mode)
		fmt.Printf("Status:    %s\n", tk.Status)
		if len(spec.Constraints) > 0 {
			fmt.Println("Constraints:")
			for _, c := range spec.Constraints {
				fmt.Printf("  - %s\n", c)
			}
		}
		if len(spec.AcceptanceCriteria) > 0 {
			fmt.Println("Acceptance criteria:")
			for _, c := range spec.AcceptanceCriteria {
				fmt.Printf("  - [%s] %s\n", c.State, c.Description)
			}
		}
		if len(spec.ValidationIntents) > 0 {
			fmt.Println("Validation intents:")
			for _, v := range spec.ValidationIntents {
				fmt.Printf("  - %s: %s\n", v.Intent, v.Command)
			}
		}
		return nil
	},
}

func init() {
	taskShowCmd.Flags().Bool("json", false, "output as JSON")
	taskCmd.AddCommand(taskShowCmd)
	rootCmd.AddCommand(taskCmd)
}
