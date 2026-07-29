/*
Copyright © 2026 Christoph Becker
*/

// Package cmd contains the Cobra command definitions for the workstreams CLI.
package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"
)

var (
	runBranch string
	runFrom   string
)

var runCmd = &cobra.Command{
	Use:   "run -- <command> [args...]",
	Short: "Run a command in a temporary workstream, then clean up",
	Long: `Create a temporary git worktree, run the given command inside it, then
remove the worktree when the command finishes (whether it succeeds or fails).

The branch is preserved after cleanup — only the worktree directory is removed.

If --branch is omitted, a unique temporary branch is created automatically
(e.g. ws-run-<timestamp>) and forked from HEAD or --from.

Use -- to separate workstreams flags from the command and its arguments:

  workstreams run -- make test
  workstreams run --branch feature/foo -- go test ./...
  workstreams run --from main -- ./scripts/ci.sh

The exit code of the command is propagated to the calling shell.`,
	Args:    cobra.MinimumNArgs(1),
	Example: "  workstreams run -- make test\n  workstreams run --branch feature/foo -- go test ./...",
	RunE: func(_ *cobra.Command, args []string) error {
		branch := runBranch
		if branch == "" {
			branch = fmt.Sprintf("ws-run-%d", time.Now().UnixNano())
		}

		if deps.wt.Exists(branch) {
			return fmt.Errorf("workstream %q already exists — use \"workstreams remove %s\" to clean it up first", branch, branch)
		}

		from := runFrom
		if from == "" {
			from = deps.cfg.DefaultBranch
		}

		createBranch := !deps.wt.BranchExists(branch)

		_, _ = fmt.Fprintf(os.Stdout, "Creating temporary workstream for branch %q...\n", branch)

		path, err := deps.wt.Add(branch, createBranch, from)
		if err != nil {
			return fmt.Errorf("create workstream: %w", err)
		}

		// exitCode is set when the command exits with a non-zero status so we
		// can call os.Exit after the cleanup defer has already run.
		var exitCode int

		// This defer runs LAST (registered first) — propagate exit code after cleanup.
		defer func() {
			if exitCode != 0 {
				os.Exit(exitCode)
			}
		}()

		// This defer runs FIRST (registered second) — always clean up the worktree.
		defer func() {
			_, _ = fmt.Fprintf(os.Stdout, "Cleaning up workstream for branch %q...\n", branch)
			if removeErr := deps.wt.Remove(branch); removeErr != nil {
				_, _ = fmt.Fprintf(os.Stderr, "warning: cleanup failed: %v\n", removeErr)
			}
		}()

		runErr := deps.runner.Run(path, branch, args)
		if runErr != nil {
			var exitErr *exec.ExitError
			if errors.As(runErr, &exitErr) {
				// Capture exit code; let defers handle cleanup then exit.
				exitCode = exitErr.ExitCode()
				return nil
			}
			return runErr
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVar(&runBranch, "branch", "", "branch to run in (created if it does not exist; default: auto-generated temp branch)")
	runCmd.Flags().StringVar(&runFrom, "from", "", "branch or commit to fork from when creating a new branch (default: HEAD)")
}
