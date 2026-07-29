/*
Copyright © 2026 Christoph Becker
*/

// Package cmd contains the Cobra command definitions for the workstreams CLI.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var shellCmd = &cobra.Command{
	Use:   "shell <branch>",
	Short: "Open an interactive shell in an existing workstream",
	Long: `Open an interactive shell inside the worktree for the given branch.
The workstream must already exist — use "workstreams new <branch>" to create one.

WORKSTREAM_BRANCH and WORKSTREAM_PATH are set in the spawned shell.`,
	Args:    cobra.ExactArgs(1),
	Example: "  workstreams shell feature/my-feature",
	RunE: func(_ *cobra.Command, args []string) error {
		branch := args[0]

		if !deps.wt.Exists(branch) {
			return fmt.Errorf("no workstream found for branch %q — create it with \"workstreams new %s\"", branch, branch)
		}

		_, _ = fmt.Fprintf(os.Stdout, "Opening shell in workstream %q\n", branch)
		_, _ = fmt.Fprintln(os.Stdout, "Type 'exit' to return.")

		return deps.spawner.Spawn(deps.wt.Path(branch), branch)
	},
}

func init() {
	rootCmd.AddCommand(shellCmd)
}
