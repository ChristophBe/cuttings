/*
Copyright © 2026 Christoph Becker
*/

// Package run holds the business logic behind the `cuttings run` command:
// deciding which kind of temporary cutting an invocation needs, whether that
// cutting is safe to remove afterwards, and running a command inside it so
// that a caught signal still leaves time to clean up.
//
// The cmd layer above it only parses arguments, prints, and asks the user
// questions.
//
// Usage:
//
//	p := run.NewProvisioner(wt, locks, cleanupOnSignal, stdout, stderr)
//	p.SweepOrphans()
//	plan, err := p.Provision(run.Spec{Branch: "feature/foo"})
//	defer p.Cleanup(plan)
package run

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/ChristophBe/cuttings/internal/worktree"
)

// Worktrees is the slice of worktree operations a run needs.
type Worktrees interface {
	Add(branch string, createBranch bool, base string) (string, error)
	AddDetached(name, base string) (string, error)
	CurrentBranch() (string, error)
	Exists(branch string) bool
	BranchExists(branch string) bool
	Path(branch string) string
	Remove(branch string, force bool) error
}

// Locks is the run-lock store used for orphan detection.
type Locks interface {
	Acquire(key, path string) error
	Release(key string) error
	Sweep(remove func(key string) error) ([]string, error)
}

// Commander runs a command inside a cutting. It is satisfied by *shell.Spawner.
type Commander interface {
	Run(ctx context.Context, dir, branch string, command []string) error
}

// Spec describes the cutting an invocation asks for.
type Spec struct {
	// Branch is the branch to run in. Empty means "a detached HEAD worktree
	// at the current branch's commit".
	Branch string
	// Source is the commit-ish to base a new worktree on. Empty means HEAD,
	// or DefaultBranch when creating a branch worktree.
	Source string
	// DefaultBranch is the configured fallback base (config's default_branch).
	DefaultBranch string
	// RemoveAfter opts a reused cutting into the temporary lifecycle.
	RemoveAfter bool
}

// Plan is a provisioned cutting together with the cleanup policy that applies
// to it.
type Plan struct {
	// Path is the worktree directory the command runs in.
	Path string
	// EnvBranch is the value exposed as CUTTING_BRANCH.
	EnvBranch string
	// Key identifies the worktree for removal and run-lock purposes.
	Key string
	// Reused is true when Provision found an existing cutting rather than
	// creating one.
	Reused bool
	// AutoRemove reports whether this cutting gets the full temporary-worktree
	// safety net: a run lock and unconditional cleanup when the run returns or
	// is signalled. A freshly created worktree always does; a reused cutting
	// only opts in via Spec.RemoveAfter — without it, removal is decided by the
	// caller's post-run prompt instead, and a signal or crash leaves the
	// cutting untouched rather than deleting real, non-temporary work.
	AutoRemove bool
}

// Provisioner creates and tears down the cuttings that `cuttings run` uses.
type Provisioner struct {
	wt              Worktrees
	locks           Locks
	cleanupOnSignal bool
	out             io.Writer
	errOut          io.Writer
	// now supplies the timestamp used to name a detached worktree. A field so
	// tests get a predictable key.
	now func() time.Time
}

// NewProvisioner returns a Provisioner writing progress to out and warnings to
// errOut. cleanupOnSignal mirrors the run_cleanup_on_signal config key: when
// false, no orphan sweeping or run-lock bookkeeping happens at all.
func NewProvisioner(wt Worktrees, locks Locks, cleanupOnSignal bool, out, errOut io.Writer) *Provisioner {
	return &Provisioner{
		wt:              wt,
		locks:           locks,
		cleanupOnSignal: cleanupOnSignal,
		out:             out,
		errOut:          errOut,
		now:             time.Now,
	}
}

// SweepOrphans removes cuttings left behind by earlier runs whose process died
// without cleaning up. It is best-effort: a failure is reported as a warning
// and never blocks the run that is about to start. It does nothing when
// cleanup-on-signal is disabled.
func (p *Provisioner) SweepOrphans() {
	if !p.cleanupOnSignal {
		return
	}
	cleaned, err := p.locks.Sweep(func(key string) error {
		if removeErr := p.wt.Remove(key, false); removeErr != nil && !errors.Is(removeErr, worktree.ErrWorktreeNotFound) {
			return removeErr
		}
		return nil
	})
	if err != nil {
		_, _ = fmt.Fprintf(p.errOut, "warning: orphan sweep failed: %v\n", err)
		return
	}
	for _, key := range cleaned {
		_, _ = fmt.Fprintf(p.out, "Cleaned up orphaned cutting from a previous run: %s\n", key)
	}
}

// Provision creates (or adopts) the cutting described by spec and records a run
// lock for it when it is subject to automatic cleanup. The returned Plan tells
// the caller where to run and how the cutting must be disposed of afterwards.
func (p *Provisioner) Provision(spec Spec) (Plan, error) {
	plan, err := p.provisionWorktree(spec)
	if err != nil {
		return Plan{}, err
	}

	plan.AutoRemove = !plan.Reused || spec.RemoveAfter
	if p.cleanupOnSignal && plan.AutoRemove {
		if lockErr := p.locks.Acquire(plan.Key, plan.Path); lockErr != nil {
			_, _ = fmt.Fprintf(p.errOut, "warning: could not record run lock: %v\n", lockErr)
		}
	}
	return plan, nil
}

// provisionWorktree picks between the three ways a run gets a directory:
// a detached worktree at the current HEAD, an existing cutting reused in
// place, or a newly created branch worktree.
func (p *Provisioner) provisionWorktree(spec Spec) (Plan, error) {
	if spec.Branch == "" {
		envBranch, err := p.wt.CurrentBranch()
		if err != nil {
			return Plan{}, fmt.Errorf("get current branch: %w", err)
		}
		key := fmt.Sprintf("cut-run-%d", p.now().UnixNano())

		_, _ = fmt.Fprintf(p.out, "Creating temporary cutting at %q...\n", envBranch)
		path, err := p.wt.AddDetached(key, spec.Source)
		if err != nil {
			return Plan{}, fmt.Errorf("create cutting: %w", err)
		}
		return Plan{Path: path, EnvBranch: envBranch, Key: key}, nil
	}

	if p.wt.Exists(spec.Branch) {
		_, _ = fmt.Fprintf(p.out, "Using existing cutting for branch %q...\n", spec.Branch)
		return Plan{
			Path:      p.wt.Path(spec.Branch),
			EnvBranch: spec.Branch,
			Key:       spec.Branch,
			Reused:    true,
		}, nil
	}

	from := spec.Source
	if from == "" {
		from = spec.DefaultBranch
	}
	createBranch := !p.wt.BranchExists(spec.Branch)

	_, _ = fmt.Fprintf(p.out, "Creating temporary cutting for branch %q...\n", spec.Branch)
	path, err := p.wt.Add(spec.Branch, createBranch, from)
	if err != nil {
		return Plan{}, fmt.Errorf("create cutting: %w", err)
	}
	return Plan{Path: path, EnvBranch: spec.Branch, Key: spec.Branch}, nil
}

// Cleanup removes the run's worktree and releases its run lock. Failures are
// reported as warnings rather than returned: a cleanup problem must not mask
// the outcome of the command the user actually asked for.
func (p *Provisioner) Cleanup(plan Plan) {
	_, _ = fmt.Fprintf(p.out, "Cleaning up cutting...\n")
	p.remove(plan.Key)
	if p.cleanupOnSignal {
		if err := p.locks.Release(plan.Key); err != nil {
			_, _ = fmt.Fprintf(p.errOut, "warning: could not remove run lock: %v\n", err)
		}
	}
}

// RemoveReused removes a reused cutting the user confirmed at the post-run
// prompt. Unlike Cleanup it touches no run lock — a reused cutting without
// --remove-after never took one.
func (p *Provisioner) RemoveReused(key string) {
	_, _ = fmt.Fprintf(p.out, "Removing cutting %q...\n", key)
	p.remove(key)
}

func (p *Provisioner) remove(key string) {
	if err := p.wt.Remove(key, false); err != nil {
		_, _ = fmt.Fprintf(p.errOut, "warning: cleanup failed: %v\n", err)
	}
}

// Outcome reports how the command a run executed ended.
type Outcome struct {
	// ExitCode is the code to propagate to the calling shell.
	ExitCode int
	// Interrupted is true when a caught signal ended the run, rather than the
	// command exiting on its own.
	Interrupted bool
}

// Invoke runs args inside plan's worktree via runner, inside a signal-aware
// context fed by sigCh. A caught signal or an *exec.ExitError both become an
// Outcome carrying a process exit code; any other error is returned for the
// caller to propagate directly.
func Invoke(runner Commander, sigCh <-chan os.Signal, plan Plan, args []string) (Outcome, error) {
	receivedSig, runErr := signalAwareRun(context.Background(), sigCh, func(ctx context.Context) error {
		return runner.Run(ctx, plan.Path, plan.EnvBranch, args)
	})

	switch {
	case runErr != nil && receivedSig != nil:
		// A caught signal takes priority over the raw process exit code: a
		// signal-killed process reports ExitCode() == -1, which loses the
		// information a shell caller expects (128+signum).
		return Outcome{ExitCode: signalExitCode(receivedSig), Interrupted: true}, nil
	case runErr != nil:
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			return Outcome{ExitCode: exitErr.ExitCode()}, nil
		}
		return Outcome{}, runErr
	case receivedSig != nil:
		// The command happened to finish on its own right as the signal
		// arrived — still honor the signal for the caller's exit code.
		return Outcome{ExitCode: signalExitCode(receivedSig), Interrupted: true}, nil
	}
	return Outcome{}, nil
}

// signalAwareRun runs fn with a context derived from base that is canceled as
// soon as a signal arrives on sigCh, and reports which signal (if any)
// triggered that cancellation. fn is expected to respect ctx cancellation (e.g.
// by passing it through to an exec.CommandContext-based runner) so that a
// caught, terminating signal still lets fn return promptly instead of Go's
// default signal disposition killing the process before any cleanup can run.
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
