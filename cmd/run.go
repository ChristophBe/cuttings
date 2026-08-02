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

// exitFn is called to terminate the process with a given exit code. It is a
// package-level variable so tests can replace it with a non-terminating stub.
var exitFn = os.Exit

var (
	runBranch string
	runFrom   string
)

var runCmd = &cobra.Command{
	Use:   "run -- <command> [args...]",
	Short: "Run a command in a temporary workstream, then clean up",
	Long: `Create a temporary git worktree, run the given command inside it, then
remove the worktree when the command finishes (whether it succeeds or fails).

Only the worktree directory is removed — no branch is created or deleted.

Without --branch, a detached HEAD worktree is created at the current branch's
HEAD commit (or --from if specified). With --branch, a worktree is created for
that branch (which is also created if it does not exist yet).

Use -- to separate workstreams flags from the command and its arguments:

  workstreams run -- make test
  workstreams run --branch feature/foo -- go test ./...
  workstreams run --from origin/main -- ./scripts/ci.sh

The exit code of the command is propagated to the calling shell.`,
	Args:    cobra.MinimumNArgs(1),
	Example: "  workstreams run -- make test\n  workstreams run --branch feature/foo -- go test ./...",
	RunE: func(_ *cobra.Command, args []string) error {
		var (
			path        string
			envBranch   string // value used for WORKSTREAM_BRANCH env var
			worktreeKey string // key used to Remove the worktree on cleanup
			err         error
		)

		if runBranch == "" {
			// No branch specified — detached HEAD at current branch's commit.
			envBranch, err = deps.wt.CurrentBranch()
			if err != nil {
				return fmt.Errorf("get current branch: %w", err)
			}
			worktreeKey = fmt.Sprintf("ws-run-%d", time.Now().UnixNano())

			_, _ = fmt.Fprintf(os.Stdout, "Creating temporary workstream at %q...\n", envBranch)
			path, err = deps.wt.AddDetached(worktreeKey, runFrom)
			if err != nil {
				return fmt.Errorf("create workstream: %w", err)
			}
		} else {
			// Explicit branch — create worktree for it (creating branch if needed).
			envBranch = runBranch
			worktreeKey = runBranch

			if deps.wt.Exists(worktreeKey) {
				return fmt.Errorf("workstream %q already exists — use \"workstreams remove %s\" to clean it up first", worktreeKey, worktreeKey)
			}

			from := runFrom
			if from == "" {
				from = deps.cfg.DefaultBranch
			}
			createBranch := !deps.wt.BranchExists(worktreeKey)

			_, _ = fmt.Fprintf(os.Stdout, "Creating temporary workstream for branch %q...\n", worktreeKey)
			path, err = deps.wt.Add(worktreeKey, createBranch, from)
			if err != nil {
				return fmt.Errorf("create workstream: %w", err)
			}
		}

		// exitCode is set when the command exits with a non-zero status so we
		// can call os.Exit after the cleanup defer has already run.
		var exitCode int

		// This defer runs LAST (registered first) — propagate exit code after cleanup.
		defer func() {
			if exitCode != 0 {
				exitFn(exitCode)
			}
		}()

		// This defer runs FIRST (registered second) — always clean up the worktree.
		defer func() {
			_, _ = fmt.Fprintf(os.Stdout, "Cleaning up workstream...\n")
			if removeErr := deps.wt.Remove(worktreeKey); removeErr != nil {
				_, _ = fmt.Fprintf(os.Stderr, "warning: cleanup failed: %v\n", removeErr)
			}
		}()

		runErr := deps.runner.Run(path, envBranch, args)
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
	runCmd.Flags().StringVar(&runBranch, "branch", "", "branch to create a worktree for (created if it does not exist)")
	runCmd.Flags().StringVar(&runFrom, "from", "", "commit-ish to base the worktree on (default: HEAD)")
	_ = runCmd.RegisterFlagCompletionFunc("branch", completeBranches)
	_ = runCmd.RegisterFlagCompletionFunc("from", completeBranches)
}
