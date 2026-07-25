package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aux-ai/aux-cli/internal/config"
	"github.com/aux-ai/aux-cli/internal/db"
	"github.com/aux-ai/aux-cli/internal/profile"
	"github.com/aux-ai/aux-cli/internal/project"
	"github.com/spf13/cobra"
)

// projectCmd is a read-only surface over resolved project identity and the
// compiled effective profile (roadmapplan.md §6.7 / §19 PR 5).
var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Inspect the resolved project identity and profile",
}

var projectShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the current project, revision, and compiled profile entries",
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

		ctx := context.Background()
		projects := project.NewService(project.NewStore(conn), project.GitVCS{})
		res, err := projects.Resolve(ctx, config.WorkingDirectory())
		if err != nil {
			return fmt.Errorf("failed to resolve project: %w", err)
		}

		pstore := profile.NewStore(conn)
		profiles := profile.NewService(pstore, profile.NewBuilder(pstore, profile.DefaultScanners()))
		version, entries, err := profiles.CompileProject(ctx, res.Project.ID, res.Root.CanonicalPath, res.Revision.VCSRevision)
		if err != nil {
			return fmt.Errorf("failed to compile profile: %w", err)
		}

		jsonOut, _ := cmd.Flags().GetBool("json")
		if jsonOut {
			out := map[string]any{
				"project":  res.Project,
				"root":     res.Root,
				"revision": res.Revision,
				"profile":  map[string]any{"version": version, "entries": entries},
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		}

		fmt.Printf("Project:  %s (%s)\n", res.Project.CanonicalName, res.Project.VCSType)
		fmt.Printf("Root:     %s\n", res.Root.CanonicalPath)
		fmt.Printf("Revision: %s", short(res.Revision.VCSRevision))
		if res.Revision.BranchName != "" {
			fmt.Printf(" (%s)", res.Revision.BranchName)
		}
		if res.Revision.DirtyTreeHash != "" {
			fmt.Printf(" [dirty]")
		}
		fmt.Println()
		fmt.Printf("Profile:  version %s (reused=%v, %d entries)\n", short(version.ID), version.Reused, len(entries))
		for _, e := range entries {
			fmt.Printf("  - [%s] %s (source=%s, confidence=%.2f)\n", e.Type, e.Key, e.SourceType, e.Confidence)
		}
		return nil
	},
}

func short(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	if s == "" {
		return "(none)"
	}
	return s
}

func init() {
	projectShowCmd.Flags().Bool("json", false, "output as JSON")
	projectCmd.AddCommand(projectShowCmd)
	rootCmd.AddCommand(projectCmd)
}
