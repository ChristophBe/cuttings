/*
Copyright © 2026 Christoph Becker
*/

package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ChristophBe/cuttings/internal/worktree"
)

func newRemoveCmd(d *Deps) *cobra.Command {
	var (
		force  bool
		all    bool
		dryRun bool
	)

	// removeAll handles "cuttings remove --all": every non-main cutting is a
	// candidate, regardless of merge status (unlike "cuttings prune", which
	// only targets merged branches). Failures removing individual cuttings
	// (e.g. uncommitted changes without --force) are collected by removeEach
	// so one bad cutting doesn't block removal of the rest.
	removeAll := func(cmd *cobra.Command) error {
		trees, err := d.wt.List()
		if err != nil {
			return err
		}

		out := cmd.OutOrStdout()
		candidates := cuttingBranches(trees)
		if len(candidates) == 0 {
			_, _ = fmt.Fprintln(out, "No cuttings to remove.")
			return nil
		}

		if dryRun {
			for _, b := range candidates {
				printWouldRemove(out, b, "")
			}
			return nil
		}

		return removeEach(out, d.wt, candidates, force)
	}

	cmd := &cobra.Command{
		Use:     "remove [branch]",
		Short:   "Uproot a cutting",
		Aliases: []string{"rm"},
		Long: `Uproot the git worktree for the given branch. The branch itself is preserved
so you can take the same cutting again later with "cuttings new <branch>".

The command will fail if the worktree has uncommitted changes. Use
"git -C .worktrees/<branch> checkout -- ." to discard them first, or pass
--force to discard them as part of removal.

Use --all to remove every cutting instead of a single branch. Combine it
with --dry-run to preview what would be removed, or --force to discard
uncommitted changes in every cutting that has them.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if all {
				return cobra.NoArgs(cmd, args)
			}
			return cobra.ExactArgs(1)(cmd, args)
		},
		ValidArgsFunction: d.completeCuttings,
		Example:           "  cuttings remove feature/my-feature\n  cuttings remove --all\n  cuttings remove --all --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			if all {
				return removeAll(cmd)
			}

			branch := args[0]

			if dryRun {
				if !d.wt.Exists(branch) {
					return fmt.Errorf("no cutting found for branch %q", branch)
				}
				printWouldRemove(cmd.OutOrStdout(), branch, "")
				return nil
			}

			if err := d.wt.Remove(branch, force); err != nil {
				if errors.Is(err, worktree.ErrWorktreeNotFound) {
					return fmt.Errorf("no cutting found for branch %q", branch)
				}
				return err
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Cutting for %q removed (branch preserved).\n", branch)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "remove even if the worktree has uncommitted or untracked changes")
	cmd.Flags().BoolVarP(&all, "all", "a", false, "remove every cutting instead of a single branch")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "show what would be removed without removing anything")
	return cmd
}
