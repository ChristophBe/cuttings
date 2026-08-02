/*
Copyright © 2026 Christoph Becker
*/

// Package cmd contains the Cobra command definitions for the workstreams CLI.
package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ChristophBe/workstreams/internal/worktree"
)

var removeCmd = &cobra.Command{
	Use:     "remove <branch>",
	Short:   "Remove a workstream worktree",
	Aliases: []string{"rm"},
	Long: `Remove the git worktree for the given branch. The branch itself is preserved
so you can re-create the workstream later with "workstreams new <branch>".

The command will fail if the worktree has uncommitted changes. Use
"git -C .worktrees/<branch> checkout -- ." to discard them first.`,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeWorkstreams,
	Example:           "  workstreams remove feature/my-feature",
	RunE: func(_ *cobra.Command, args []string) error {
		branch := args[0]

		if err := deps.wt.Remove(branch); err != nil {
			if errors.Is(err, worktree.ErrWorktreeNotFound) {
				return fmt.Errorf("no workstream found for branch %q", branch)
			}
			return err
		}

		_, _ = fmt.Fprintf(os.Stdout, "Workstream for %q removed (branch preserved).\n", branch)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(removeCmd)
}
