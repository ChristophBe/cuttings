/*
Copyright © 2026 Christoph Becker
*/

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newNewCmd(d *Deps) *cobra.Command {
	var sourceBranch string

	cmd := &cobra.Command{
		Use:   "new <branch>",
		Short: "Take a new cutting and open an interactive shell",
		Long: `Take a new git worktree for the given branch and open an interactive
shell inside it. If the branch does not exist it will be created.

The worktree is stored at .worktrees/<branch>/ relative to the repository root.
Two environment variables are set inside the shell:

  CUTTING_BRANCH  the name of the branch
  CUTTING_PATH    the absolute path to the worktree directory

Use --source to specify the branch or commit to fork from when creating a new
branch. If omitted, the new branch is created from HEAD.

Exiting the shell removes you from the cutting but does not delete it.
Use "cuttings remove <branch>" to clean up.`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: d.completeBranches,
		Example:           "  cuttings new feature/my-feature\n  cuttings new feature/my-feature --source main",
		RunE: func(cmd *cobra.Command, args []string) error {
			branch := args[0]

			if d.wt.Exists(branch) {
				return fmt.Errorf("cutting %q already exists — use \"cuttings shell %s\" to re-enter it", branch, branch)
			}

			out := cmd.OutOrStdout()
			_, _ = fmt.Fprintf(out, "Creating cutting for branch %q...\n", branch)

			from := sourceBranch
			if from == "" {
				from = d.cfg.DefaultBranch
			}

			createBranch := !d.wt.BranchExists(branch)
			path, err := d.wt.Add(branch, createBranch, from)
			if err != nil {
				return fmt.Errorf("create cutting: %w", err)
			}

			_, _ = fmt.Fprintf(out, "Cutting ready at %s\n", path)
			_, _ = fmt.Fprintln(out, "Opening shell — type 'exit' to return.")

			return d.spawner.Spawn(path, branch)
		},
	}

	cmd.Flags().StringVarP(&sourceBranch, "source", "s", "", "branch or commit to fork from when creating a new branch (default: HEAD)")
	_ = cmd.RegisterFlagCompletionFunc("source", d.completeBranches)
	return cmd
}
