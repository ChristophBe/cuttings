/*
Copyright © 2026 Christoph Becker
*/

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newPruneCmd(d *Deps) *cobra.Command {
	var (
		force  bool
		dryRun bool
	)

	cmd := &cobra.Command{
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
		RunE: func(cmd *cobra.Command, _ []string) error {
			base := d.cfg.DefaultBranch
			if base == "" {
				var err error
				base, err = d.wt.CurrentBranch()
				if err != nil {
					return err
				}
			}

			merged, err := d.wt.ListMergedBranches(base)
			if err != nil {
				return err
			}
			mergedSet := make(map[string]bool, len(merged))
			for _, b := range merged {
				mergedSet[b] = true
			}

			trees, err := d.wt.List()
			if err != nil {
				return err
			}

			var candidates []string
			for _, b := range cuttingBranches(trees) {
				if mergedSet[b] {
					candidates = append(candidates, b)
				}
			}

			out := cmd.OutOrStdout()
			if len(candidates) == 0 {
				_, _ = fmt.Fprintln(out, "No cuttings to prune.")
				return nil
			}

			if dryRun {
				for _, b := range candidates {
					printWouldRemove(out, b, fmt.Sprintf("merged into %q", base))
				}
				return nil
			}

			return removeEach(out, d.wt, candidates, force)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "remove even if a cutting has uncommitted or untracked changes")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "show what would be removed without removing anything")
	return cmd
}
