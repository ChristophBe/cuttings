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

var sourceBranch string

var newCmd = &cobra.Command{
	Use:   "new <branch>",
	Short: "Create a new cutting and open an interactive shell",
	Long: `Create a new git worktree for the given branch and open an interactive
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
	ValidArgsFunction: completeBranches,
	Example:           "  cuttings new feature/my-feature\n  cuttings new feature/my-feature --source main",
	RunE: func(_ *cobra.Command, args []string) error {
		branch := args[0]

		if deps.wt.Exists(branch) {
			return fmt.Errorf("cutting %q already exists — use \"cuttings shell %s\" to re-enter it", branch, branch)
		}

		_, _ = fmt.Fprintf(os.Stdout, "Creating cutting for branch %q...\n", branch)

		from := sourceBranch
		if from == "" {
			from = deps.cfg.DefaultBranch
		}

		createBranch := !deps.wt.BranchExists(branch)
		path, err := deps.wt.Add(branch, createBranch, from)
		if err != nil {
			return fmt.Errorf("create cutting: %w", err)
		}

		_, _ = fmt.Fprintf(os.Stdout, "Cutting ready at %s\n", path)
		_, _ = fmt.Fprintln(os.Stdout, "Opening shell — type 'exit' to return.")

		return deps.spawner.Spawn(path, branch)
	},
}

func init() {
	rootCmd.AddCommand(newCmd)
	newCmd.Flags().StringVarP(&sourceBranch, "source", "s", "", "branch or commit to fork from when creating a new branch (default: HEAD)")
	_ = newCmd.RegisterFlagCompletionFunc("source", completeBranches)
}
