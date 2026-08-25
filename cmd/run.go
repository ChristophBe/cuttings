/*
Copyright © 2026 Christoph Becker
*/

// Package cmd contains the Cobra command definitions for the workstreams CLI.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
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

// signalAwareRun runs fn with a context derived from base that is canceled
// as soon as a signal arrives on sigCh, and reports which signal (if any)
// triggered that cancellation. fn is expected to respect ctx cancellation
// (e.g. by passing it through to an exec.CommandContext-based runner) so
// that a caught, terminating signal still lets fn return promptly instead of
// Go's default signal disposition killing the process before any cleanup
// defers can run.
func signalAwareRun(base context.Context, sigCh <-chan os.Signal, fn func(ctx context.Context) error) (receivedSig os.Signal, runErr error) {
	ctx, cancel := context.WithCancel(base)
	defer cancel()

	stop := make(chan struct{})
	watcherDone := make(chan struct{})
	go func() {
		defer close(watcherDone)
		select {
		case sig := <-sigCh:
			receivedSig = sig
			cancel()
		case <-stop:
		}
	}()

	runErr = fn(ctx)
	close(stop)
	<-watcherDone // wait for the watcher to finish before reading receivedSig
	return receivedSig, runErr
}

// signalExitCode maps a terminating signal to the shell convention of
// 128+signum, matching what a shell itself reports for a signal-killed
// foreground process (e.g. 130 for SIGINT, 143 for SIGTERM).
func signalExitCode(sig os.Signal) int {
	if s, ok := sig.(syscall.Signal); ok {
		return 128 + int(s)
	}
	return 1
}

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
		cleanupOnSignal := deps.cfg.RunCleanupOnSignal

		if cleanupOnSignal {
			if cleaned, sweepErr := deps.wt.SweepOrphans(); sweepErr != nil {
				_, _ = fmt.Fprintf(os.Stderr, "warning: orphan sweep failed: %v\n", sweepErr)
			} else {
				for _, key := range cleaned {
					_, _ = fmt.Fprintf(os.Stdout, "Cleaned up orphaned workstream from a previous run: %s\n", key)
				}
			}
		}

		// sigCh is only ever written to when cleanupOnSignal is true; left
		// unregistered otherwise so run falls back to plain defer-only cleanup
		// (matching Go's default, uncaught signal disposition).
		sigCh := make(chan os.Signal, 1)
		if cleanupOnSignal {
			signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
			defer signal.Stop(sigCh)
		}

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

		if cleanupOnSignal {
			if lockErr := deps.wt.Lock(worktreeKey); lockErr != nil {
				_, _ = fmt.Fprintf(os.Stderr, "warning: could not record run lock: %v\n", lockErr)
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
			if removeErr := deps.wt.Remove(worktreeKey, false); removeErr != nil {
				_, _ = fmt.Fprintf(os.Stderr, "warning: cleanup failed: %v\n", removeErr)
			}
			if cleanupOnSignal {
				if unlockErr := deps.wt.Unlock(worktreeKey); unlockErr != nil {
					_, _ = fmt.Fprintf(os.Stderr, "warning: could not remove run lock: %v\n", unlockErr)
				}
			}
		}()

		receivedSig, runErr := signalAwareRun(context.Background(), sigCh, func(ctx context.Context) error {
			return deps.runner.Run(ctx, path, envBranch, args)
		})
		if runErr != nil {
			// A caught signal takes priority over the raw process exit code: a
			// signal-killed process reports ExitCode() == -1, which loses the
			// information a shell caller expects (128+signum).
			if receivedSig != nil {
				exitCode = signalExitCode(receivedSig)
				return nil
			}
			var exitErr *exec.ExitError
			if errors.As(runErr, &exitErr) {
				// Capture exit code; let defers handle cleanup then exit.
				exitCode = exitErr.ExitCode()
				return nil
			}
			return runErr
		}
		if receivedSig != nil {
			// The command happened to finish on its own right as the signal
			// arrived — still honor the signal for the caller's exit code.
			exitCode = signalExitCode(receivedSig)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVarP(&runBranch, "branch", "b", "", "branch to create a worktree for (created if it does not exist)")
	runCmd.Flags().StringVarP(&runFrom, "from", "f", "", "commit-ish to base the worktree on (default: HEAD)")
	_ = runCmd.RegisterFlagCompletionFunc("branch", completeBranches)
	_ = runCmd.RegisterFlagCompletionFunc("from", completeBranches)
}
