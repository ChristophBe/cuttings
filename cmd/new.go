/*
Copyright © 2026 Christoph Becker
*/

// Package cmd contains the Cobra command definitions for the workstreams CLI.
package cmd

import (
	"fmt"
	"os"

	"github.com/ChristophBe/workstreams/internal/config"
	"github.com/ChristophBe/workstreams/internal/shell"
	"github.com/ChristophBe/workstreams/internal/worktree"
	"github.com/spf13/cobra"
)

var fromBranch string

var newCmd = &cobra.Command{
	Use:   "new <branch>",
	Short: "Create a new workstream and open an interactive shell",
	Long: `Create a new git worktree for the given branch and open an interactive
shell inside it. If the branch does not exist it will be created.

The worktree is stored at .worktrees/<branch>/ relative to the repository root.
Two environment variables are set inside the shell:

  WORKSTREAM_BRANCH  the name of the branch
  WORKSTREAM_PATH    the absolute path to the worktree directory

Use --from to specify the branch or commit to fork from when creating a new
branch. If omitted, the new branch is created from HEAD.

Exiting the shell removes you from the workstream but does not delete it.
Use "workstreams remove <branch>" to clean up.`,
	Args:    cobra.ExactArgs(1),
	Example: "  workstreams new feature/my-feature\n  workstreams new feature/my-feature --from main",
	RunE: func(_ *cobra.Command, args []string) error {
		branch := args[0]

		repoRoot, err := worktree.FindRepoRoot()
		if err != nil {
			return err
		}

		if worktree.Exists(repoRoot, config.WorktreesDir(), branch) {
			return fmt.Errorf("workstream %q already exists — use \"workstreams shell %s\" to re-enter it", branch, branch)
		}

		_, _ = fmt.Fprintf(os.Stdout, "Creating workstream for branch %q...\n", branch)

		from := fromBranch
		if from == "" {
			from = config.DefaultBranch()
		}

		createBranch := !branchExists(repoRoot, branch)
		path, err := worktree.Add(repoRoot, config.WorktreesDir(), branch, createBranch, from)
		if err != nil {
			return fmt.Errorf("create workstream: %w", err)
		}

		_, _ = fmt.Fprintf(os.Stdout, "Workstream ready at %s\n", path)
		_, _ = fmt.Fprintln(os.Stdout, "Opening shell — type 'exit' to return.")

		return shell.Spawn(path, branch)
	},
}

func init() {
	rootCmd.AddCommand(newCmd)
	newCmd.Flags().StringVar(&fromBranch, "from", "", "branch or commit to fork from when creating a new branch (default: HEAD)")
}
