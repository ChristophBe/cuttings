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
)

var (
	pruneForce  bool
	pruneDryRun bool
)

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Clear away cuttings whose branch is fully merged",
	Long: `Remove every cutting whose branch has already been fully merged into the
default branch — or, if default_branch is not configured, the branch
currently checked out in the main worktree. The branches themselves are
preserved, same as "cuttings remove".

Cuttings with uncommitted or untracked changes are left in place unless
--force is given. Use --dry-run to see what would be removed without
removing anything.`,
	Example: "  cuttings prune\n  cuttings prune --dry-run\n  cuttings prune --force",
	RunE: func(_ *cobra.Command, _ []string) error {
		base := deps.cfg.DefaultBranch
		if base == "" {
			var err error
			base, err = deps.wt.CurrentBranch()
			if err != nil {
				return err
			}
		}

		merged, err := deps.wt.ListMergedBranches(base)
		if err != nil {
			return err
		}
		mergedSet := make(map[string]bool, len(merged))
		for _, b := range merged {
			mergedSet[b] = true
		}

		trees, err := deps.wt.List()
		if err != nil {
			return err
		}

		var candidates []string
		for _, t := range trees {
			if t.IsMain || t.Branch == "" {
				continue
			}
			if mergedSet[t.Branch] {
				candidates = append(candidates, t.Branch)
			}
		}

		if len(candidates) == 0 {
			_, _ = fmt.Fprintln(os.Stdout, "No cuttings to prune.")
			return nil
		}

		if pruneDryRun {
			for _, b := range candidates {
				_, _ = fmt.Fprintf(os.Stdout, "Would remove cutting for %q (merged into %q).\n", b, base)
			}
			return nil
		}

		var errs []error
		for _, b := range candidates {
			if err := deps.wt.Remove(b, pruneForce); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", b, err))
				continue
			}
			_, _ = fmt.Fprintf(os.Stdout, "Cutting for %q removed (branch preserved).\n", b)
		}
		return errors.Join(errs...)
	},
}

func init() {
	pruneCmd.Flags().BoolVarP(&pruneForce, "force", "f", false, "remove even if a cutting has uncommitted or untracked changes")
	pruneCmd.Flags().BoolVarP(&pruneDryRun, "dry-run", "n", false, "show what would be removed without removing anything")
	rootCmd.AddCommand(pruneCmd)
}
