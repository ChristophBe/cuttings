/*
Copyright © 2026 Christoph Becker
*/

// Package cmd contains the Cobra command definitions for the cuttings CLI.
package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// exitFn is called to terminate the process with a given exit code. It is a
// package-level variable so tests can replace it with a non-terminating stub.
var exitFn = os.Exit

// promptReader is the source read from when asking whether to remove a
// reused cutting. A package-level variable so tests can inject scripted
// answers instead of the real os.Stdin.
var promptReader io.Reader = os.Stdin

var (
	runBranch      string
	runSource      string
	runRemoveAfter bool
	runInPlace     bool
)

// confirmRemoval asks the user whether the reused cutting for branch
// should be removed, reading one line from promptReader. Any answer other
// than "y"/"yes" (including EOF, e.g. no terminal attached) is treated as
// "no" — the safe default that never silently deletes existing work.
func confirmRemoval(branch string) bool {
	_, _ = fmt.Fprintf(os.Stdout, "Remove cutting %q? [y/N]: ", branch)
	line, _ := bufio.NewReader(promptReader).ReadString('\n')
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}

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

// runAndExitCode runs args via deps.runner inside a signal-aware context
// (sigCh, as set up by the caller), and maps the outcome to an exit code:
// a caught signal or a *exec.ExitError both become a process exit code
// (interrupted distinguishes the former, e.g. so a caller can skip a
// post-run prompt when the run was cut short); any other error is returned
// as runFailErr for the caller to propagate directly.
func runAndExitCode(sigCh <-chan os.Signal, path, envBranch string, args []string) (exitCode int, interrupted bool, runFailErr error) {
	receivedSig, runErr := signalAwareRun(context.Background(), sigCh, func(ctx context.Context) error {
		return deps.runner.Run(ctx, path, envBranch, args)
	})

	switch {
	case runErr != nil && receivedSig != nil:
		// A caught signal takes priority over the raw process exit code: a
		// signal-killed process reports ExitCode() == -1, which loses the
		// information a shell caller expects (128+signum).
		exitCode = signalExitCode(receivedSig)
		interrupted = true
	case runErr != nil:
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			runFailErr = runErr
		}
	case receivedSig != nil:
		// The command happened to finish on its own right as the signal
		// arrived — still honor the signal for the caller's exit code.
		exitCode = signalExitCode(receivedSig)
		interrupted = true
	}
	return exitCode, interrupted, runFailErr
}

// runInPlaceCommand runs args directly in the current worktree (the one
// cuttings was invoked from — the main repo checkout or a linked cutting)
// without creating, reusing, locking, or removing any cutting. It is used
// when --in-place is set, for e.g. running a long-lived dev server against the
// exact files being edited live in another shell.
func runInPlaceCommand(args []string) error {
	cleanupOnSignal := deps.cfg.RunCleanupOnSignal

	sigCh := make(chan os.Signal, 1)
	if cleanupOnSignal {
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
		defer signal.Stop(sigCh)
	}

	envBranch, err := deps.wt.CurrentBranch()
	if err != nil {
		return fmt.Errorf("get current branch: %w", err)
	}
	path := deps.wt.RepoRoot()

	var exitCode int
	defer func() {
		if exitCode != 0 {
			exitFn(exitCode)
		}
	}()

	var runFailErr error
	exitCode, _, runFailErr = runAndExitCode(sigCh, path, envBranch, args)
	return runFailErr
}

var runCmd = &cobra.Command{
	Use:   "run -- <command> [args...]",
	Short: "Run a command in a temporary cutting, then clear it away",
	Long: `Take a temporary cutting, run the given command inside it, then
clear it away when the command finishes (whether it succeeds or fails).

Only the worktree directory is removed — no branch is created or deleted.

Without --branch, a detached HEAD worktree is created at the current branch's
HEAD commit (or --source if specified). With --branch, a worktree is created for
that branch (which is also created if it does not exist yet).

If --branch names a cutting that already exists, its worktree is reused
in place (nothing is created) instead of failing. Since a reused cutting
isn't temporary, it is not removed automatically: once the command finishes,
you are asked whether to remove it. Use --remove-after to skip that prompt
and always remove it, e.g. from a script or CI.

Use --in-place to run the command directly in the worktree cuttings was invoked
from (the main repo checkout, or a cutting you cd'd into) instead of
creating or reusing any cutting — nothing is created, locked, or removed
afterward, regardless of uncommitted changes. This is for e.g. running a
dev server against the exact files being edited live in another shell.
--in-place cannot be combined with --branch, --source, or --remove-after.

Use -- to separate cuttings flags from the command and its arguments:

  cuttings run -- make test
  cuttings run --branch feature/foo -- go test ./...
  cuttings run --source origin/main -- ./scripts/ci.sh
  cuttings run --branch feature/foo --remove-after -- go test ./...
  cuttings run --in-place -- npm run dev

The exit code of the command is propagated to the calling shell.`,
	Args: cobra.MinimumNArgs(1),
	Example: "  cuttings run -- make test\n  cuttings run --branch feature/foo -- go test ./...\n" +
		"  cuttings run --branch feature/foo --remove-after -- go test ./...\n" +
		"  cuttings run --in-place -- npm run dev",
	RunE: func(_ *cobra.Command, args []string) error {
		if runInPlace {
			return runInPlaceCommand(args)
		}

		cleanupOnSignal := deps.cfg.RunCleanupOnSignal

		if cleanupOnSignal {
			if cleaned, sweepErr := deps.wt.SweepOrphans(); sweepErr != nil {
				_, _ = fmt.Fprintf(os.Stderr, "warning: orphan sweep failed: %v\n", sweepErr)
			} else {
				for _, key := range cleaned {
					_, _ = fmt.Fprintf(os.Stdout, "Cleaned up orphaned cutting from a previous run: %s\n", key)
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
			path            string
			envBranch       string // value used for CUTTING_BRANCH env var
			worktreeKey     string // key used to Remove the worktree on cleanup
			reusingExisting bool   // true when --branch names a cutting that already exists
			err             error
		)

		if runBranch == "" {
			// No branch specified — detached HEAD at current branch's commit.
			envBranch, err = deps.wt.CurrentBranch()
			if err != nil {
				return fmt.Errorf("get current branch: %w", err)
			}
			worktreeKey = fmt.Sprintf("cut-run-%d", time.Now().UnixNano())

			_, _ = fmt.Fprintf(os.Stdout, "Creating temporary cutting at %q...\n", envBranch)
			path, err = deps.wt.AddDetached(worktreeKey, runSource)
			if err != nil {
				return fmt.Errorf("create cutting: %w", err)
			}
		} else {
			// Explicit branch.
			envBranch = runBranch
			worktreeKey = runBranch

			if deps.wt.Exists(worktreeKey) {
				reusingExisting = true
				path = deps.wt.Path(worktreeKey)
				_, _ = fmt.Fprintf(os.Stdout, "Using existing cutting for branch %q...\n", worktreeKey)
			} else {
				from := runSource
				if from == "" {
					from = deps.cfg.DefaultBranch
				}
				createBranch := !deps.wt.BranchExists(worktreeKey)

				_, _ = fmt.Fprintf(os.Stdout, "Creating temporary cutting for branch %q...\n", worktreeKey)
				path, err = deps.wt.Add(worktreeKey, createBranch, from)
				if err != nil {
					return fmt.Errorf("create cutting: %w", err)
				}
			}
		}

		// willAutoRemove decides whether this run's worktree gets the full
		// temporary-worktree safety net (run lock, unconditional cleanup on
		// return or signal). A freshly-created worktree always gets it; a
		// reused existing cutting only opts in via --remove-after — without
		// it, removal is instead decided by the post-run prompt below, and a
		// signal or crash leaves the cutting untouched rather than deleting
		// real, non-temporary work.
		willAutoRemove := !reusingExisting || runRemoveAfter

		if cleanupOnSignal && willAutoRemove {
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

		if willAutoRemove {
			// This defer runs FIRST (registered second) — always clean up the worktree.
			defer func() {
				_, _ = fmt.Fprintf(os.Stdout, "Cleaning up cutting...\n")
				if removeErr := deps.wt.Remove(worktreeKey, false); removeErr != nil {
					_, _ = fmt.Fprintf(os.Stderr, "warning: cleanup failed: %v\n", removeErr)
				}
				if cleanupOnSignal {
					if unlockErr := deps.wt.Unlock(worktreeKey); unlockErr != nil {
						_, _ = fmt.Fprintf(os.Stderr, "warning: could not remove run lock: %v\n", unlockErr)
					}
				}
			}()
		}

		var (
			interrupted bool
			runFailErr  error
		)
		exitCode, interrupted, runFailErr = runAndExitCode(sigCh, path, envBranch, args)

		// Reused cuttings that didn't opt into --remove-after are never
		// touched by the defer above; decide their fate here instead, once the
		// command has actually completed (not merely been interrupted).
		if reusingExisting && !runRemoveAfter && !interrupted {
			if confirmRemoval(worktreeKey) {
				_, _ = fmt.Fprintf(os.Stdout, "Removing cutting %q...\n", worktreeKey)
				if removeErr := deps.wt.Remove(worktreeKey, false); removeErr != nil {
					_, _ = fmt.Fprintf(os.Stderr, "warning: cleanup failed: %v\n", removeErr)
				}
			} else {
				_, _ = fmt.Fprintf(os.Stdout, "Leaving cutting %q in place.\n", worktreeKey)
			}
		}

		if runFailErr != nil {
			return runFailErr
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVarP(&runBranch, "branch", "b", "", "branch to create a worktree for (created if it does not exist; reused if it does)")
	runCmd.Flags().StringVarP(&runSource, "source", "s", "", "commit-ish to base the worktree on (default: HEAD)")
	runCmd.Flags().BoolVarP(&runRemoveAfter, "remove-after", "r", false, "when reusing an existing --branch cutting, remove it after the command finishes without prompting")
	runCmd.Flags().BoolVarP(&runInPlace, "in-place", "i", false, "run directly in the current worktree instead of creating or reusing a cutting; nothing is created, locked, or removed")
	runCmd.MarkFlagsMutuallyExclusive("in-place", "branch")
	runCmd.MarkFlagsMutuallyExclusive("in-place", "source")
	runCmd.MarkFlagsMutuallyExclusive("in-place", "remove-after")
	_ = runCmd.RegisterFlagCompletionFunc("branch", completeBranches)
	_ = runCmd.RegisterFlagCompletionFunc("source", completeBranches)
}
