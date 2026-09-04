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

var (
	removeForce  bool
	removeAll    bool
	removeDryRun bool
)

var removeCmd = &cobra.Command{
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
		if removeAll {
			return cobra.NoArgs(cmd, args)
		}
		return cobra.ExactArgs(1)(cmd, args)
	},
	ValidArgsFunction: completeCuttings,
	Example:           "  cuttings remove feature/my-feature\n  cuttings remove --all\n  cuttings remove --all --dry-run",
	RunE: func(_ *cobra.Command, args []string) error {
		if removeAll {
			return removeAllCuttings()
		}

		branch := args[0]

		if removeDryRun {
			if !deps.wt.Exists(branch) {
				return fmt.Errorf("no cutting found for branch %q", branch)
			}
			_, _ = fmt.Fprintf(os.Stdout, "Would remove cutting for %q.\n", branch)
			return nil
		}

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

// removeAllCuttings handles "cuttings remove --all": every non-main cutting
// is a candidate, regardless of merge status (unlike "cuttings prune", which
// only targets merged branches). Failures removing individual cuttings
// (e.g. uncommitted changes without --force) are collected so one bad
// cutting doesn't block removal of the rest.
func removeAllCuttings() error {
	trees, err := deps.wt.List()
	if err != nil {
		return err
	}

	var candidates []string
	for _, t := range trees {
		if t.IsMain || t.Branch == "" {
			continue
		}
		candidates = append(candidates, t.Branch)
	}

	if len(candidates) == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "No cuttings to remove.")
		return nil
	}

	if removeDryRun {
		for _, b := range candidates {
			_, _ = fmt.Fprintf(os.Stdout, "Would remove cutting for %q.\n", b)
		}
		return nil
	}

	var errs []error
	for _, b := range candidates {
		if err := deps.wt.Remove(b, removeForce); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", b, err))
			continue
		}
		_, _ = fmt.Fprintf(os.Stdout, "Cutting for %q removed (branch preserved).\n", b)
	}
	return errors.Join(errs...)
}

func init() {
	removeCmd.Flags().BoolVarP(&removeForce, "force", "f", false, "remove even if the worktree has uncommitted or untracked changes")
	removeCmd.Flags().BoolVarP(&removeAll, "all", "a", false, "remove every cutting instead of a single branch")
	removeCmd.Flags().BoolVarP(&removeDryRun, "dry-run", "n", false, "show what would be removed without removing anything")
	rootCmd.AddCommand(removeCmd)
}
