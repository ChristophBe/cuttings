/*
Copyright © 2026 Christoph Becker
*/

// Package cmd contains the Cobra command definitions for the cuttings CLI.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var shellCmd = &cobra.Command{
	Use:   "shell <branch>",
	Short: "Open an interactive shell in an existing cutting",
	Long: `Open an interactive shell inside the worktree for the given branch.
The cutting must already exist — use "cuttings new <branch>" to create one.

CUTTING_BRANCH and CUTTING_PATH are set in the spawned shell.`,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeCuttings,
	Example:           "  cuttings shell feature/my-feature",
	RunE: func(_ *cobra.Command, args []string) error {
		branch := args[0]

		if !deps.wt.Exists(branch) {
			return fmt.Errorf("no cutting found for branch %q — create it with \"cuttings new %s\"", branch, branch)
		}

		_, _ = fmt.Fprintf(os.Stdout, "Opening shell in cutting %q\n", branch)
		_, _ = fmt.Fprintln(os.Stdout, "Type 'exit' to return.")

		return deps.spawner.Spawn(deps.wt.Path(branch), branch)
	},
}

func init() {
	rootCmd.AddCommand(shellCmd)
}
