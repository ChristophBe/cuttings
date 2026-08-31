/*
Copyright © 2026 Christoph Becker
*/

// Package cmd contains the Cobra command definitions for the cuttings CLI.
package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ChristophBe/cuttings/internal/worktree"
)

var removeForce bool

var removeCmd = &cobra.Command{
	Use:     "remove <branch>",
	Short:   "Remove a cutting worktree",
	Aliases: []string{"rm"},
	Long: `Remove the git worktree for the given branch. The branch itself is preserved
so you can re-create the cutting later with "cuttings new <branch>".

The command will fail if the worktree has uncommitted changes. Use
"git -C .worktrees/<branch> checkout -- ." to discard them first, or pass
--force to discard them as part of removal.`,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeCuttings,
	Example:           "  cuttings remove feature/my-feature",
	RunE: func(_ *cobra.Command, args []string) error {
		branch := args[0]

		if err := deps.wt.Remove(branch, removeForce); err != nil {
			if errors.Is(err, worktree.ErrWorktreeNotFound) {
				return fmt.Errorf("no cutting found for branch %q", branch)
			}
			return err
		}

		_, _ = fmt.Fprintf(os.Stdout, "Cutting for %q removed (branch preserved).\n", branch)
		return nil
	},
}

func init() {
	removeCmd.Flags().BoolVarP(&removeForce, "force", "f", false, "remove even if the worktree has uncommitted or untracked changes")
	rootCmd.AddCommand(removeCmd)
}
