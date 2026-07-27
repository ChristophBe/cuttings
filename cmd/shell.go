/*
Copyright © 2026 Christoph Becker
*/

// Package cmd contains the Cobra command definitions for the workstreams CLI.
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ChristophBe/workstreams/internal/shell"
	"github.com/ChristophBe/workstreams/internal/worktree"
	"github.com/spf13/cobra"
)

var shellCmd = &cobra.Command{
	Use:   "shell <branch>",
	Short: "Open an interactive shell in an existing workstream",
	Long: `Open an interactive shell inside the worktree for the given branch.
The workstream must already exist — use "workstreams new <branch>" to create one.

WORKSTREAM_BRANCH and WORKSTREAM_PATH are set in the spawned shell.`,
	Args:    cobra.ExactArgs(1),
	Example: "  workstreams shell feature/my-feature",
	RunE: func(_ *cobra.Command, args []string) error {
		branch := args[0]

		repoRoot, err := worktree.FindRepoRoot()
		if err != nil {
			return err
		}

		if !worktree.Exists(repoRoot, branch) {
			return fmt.Errorf("no workstream found for branch %q — create it with \"workstreams new %s\"", branch, branch)
		}

		// The worktree path mirrors the structure from worktree.Add.
		path := filepath.Join(repoRoot, ".worktrees", filepath.FromSlash(branch))

		_, _ = fmt.Fprintf(os.Stdout, "Opening shell in workstream %q\n", branch)
		_, _ = fmt.Fprintln(os.Stdout, "Type 'exit' to return.")

		return shell.Spawn(path, branch)
	},
}

func init() {
	rootCmd.AddCommand(shellCmd)
}

// branchExists reports whether branch already exists in the repository at repoRoot.
func branchExists(repoRoot, branch string) bool {
	//nolint:gosec // branch is a user-supplied git ref; this is intentional.
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	cmd.Dir = repoRoot
	// Also check remote tracking branches with sanitised names.
	if cmd.Run() == nil {
		return true
	}
	// Try remote refs: refs/remotes/origin/<branch>
	//nolint:gosec // branch is a user-supplied git ref; this is intentional.
	cmd2 := exec.Command("git", "show-ref", "--verify", "--quiet",
		"refs/remotes/origin/"+strings.TrimPrefix(branch, "origin/"))
	cmd2.Dir = repoRoot
	return cmd2.Run() == nil
}
