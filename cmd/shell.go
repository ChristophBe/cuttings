/*
Copyright © 2026 Christoph Becker
*/

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newShellCmd(d *Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "shell <branch>",
		Short: "Open an interactive shell in an existing cutting",
		Long: `Open an interactive shell inside the worktree for the given branch.
The cutting must already exist — use "cuttings new <branch>" to create one.

CUTTING_BRANCH and CUTTING_PATH are set in the spawned shell.`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: d.completeCuttings,
		Example:           "  cuttings shell feature/my-feature",
		RunE: func(cmd *cobra.Command, args []string) error {
			branch := args[0]

			if !d.wt.Exists(branch) {
				return fmt.Errorf("no cutting found for branch %q — create it with \"cuttings new %s\"", branch, branch)
			}

			out := cmd.OutOrStdout()
			_, _ = fmt.Fprintf(out, "Opening shell in cutting %q\n", branch)
			_, _ = fmt.Fprintln(out, "Type 'exit' to return.")

			return d.spawner.Spawn(d.wt.Path(branch), branch)
		},
	}
}
