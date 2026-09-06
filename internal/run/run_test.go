/*
Copyright © 2026 Christoph Becker
*/
package run_test

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/ChristophBe/cuttings/internal/run"
	"github.com/ChristophBe/cuttings/internal/worktree"
)

type stubWorktrees struct {
	addPath          string
	addErr           error
	addDetachedPath  string
	addDetachedErr   error
	currentBranch    string
	currentBranchErr error
	exists           bool
	branchExists     bool
	path             string
	removeErr        error
	removeErrs       map[string]error

	addBranch       string
	addCreateBranch bool
	addBase         string
	addDetachedName string
	addDetachedBase string
	removed         []string
}

func (s *stubWorktrees) Add(branch string, createBranch bool, base string) (string, error) {
	s.addBranch, s.addCreateBranch, s.addBase = branch, createBranch, base
	return s.addPath, s.addErr
}

func (s *stubWorktrees) AddDetached(name, base string) (string, error) {
	s.addDetachedName, s.addDetachedBase = name, base
	return s.addDetachedPath, s.addDetachedErr
}
func (s *stubWorktrees) CurrentBranch() (string, error) { return s.currentBranch, s.currentBranchErr }
func (s *stubWorktrees) Exists(string) bool             { return s.exists }
func (s *stubWorktrees) BranchExists(string) bool       { return s.branchExists }
func (s *stubWorktrees) Path(string) string             { return s.path }

func (s *stubWorktrees) Remove(branch string, _ bool) error {
	s.removed = append(s.removed, branch)
	if err, ok := s.removeErrs[branch]; ok {
		return err
	}
	return s.removeErr
}

type stubLocks struct {
	acquireErr  error
	releaseErr  error
	sweepResult []string
	sweepErr    error

	acquireKey    string
	acquirePath   string
	releaseKey    string
	releaseCalled bool
	sweepCalled   bool
	sweepRemove   func(string) error
}

func (s *stubLocks) Acquire(key, path string) error {
	s.acquireKey, s.acquirePath = key, path
	return s.acquireErr
}

func (s *stubLocks) Release(key string) error {
	s.releaseCalled = true
	s.releaseKey = key
	return s.releaseErr
}

func (s *stubLocks) Sweep(remove func(string) error) ([]string, error) {
	s.sweepCalled = true
	s.sweepRemove = remove
	return s.sweepResult, s.sweepErr
}

type fixture struct {
	wt     *stubWorktrees
	locks  *stubLocks
	out    bytes.Buffer
	errOut bytes.Buffer
	p      *run.Provisioner
}

func newFixture(t *testing.T, wt *stubWorktrees, cleanupOnSignal bool) *fixture {
	t.Helper()
	f := &fixture{wt: wt, locks: &stubLocks{}}
	f.p = run.NewProvisioner(wt, f.locks, cleanupOnSignal, &f.out, &f.errOut)
	return f
}

// --- Provision: detached HEAD (no branch given) ---

func TestProvision_NoBranch_CreatesDetachedWorktree(t *testing.T) {
	t.Parallel()

	f := newFixture(t, &stubWorktrees{currentBranch: "feature/foo", addDetachedPath: "/tmp/ws"}, true)

	plan, err := f.p.Provision(run.Spec{Source: "origin/main"})
	if err != nil {
		t.Fatalf("Provision() unexpected error: %v", err)
	}

	if plan.EnvBranch != "feature/foo" {
		t.Errorf("EnvBranch = %q, want the current branch", plan.EnvBranch)
	}
	if plan.Path != "/tmp/ws" {
		t.Errorf("Path = %q, want the path AddDetached returned", plan.Path)
	}
	if !strings.HasPrefix(plan.Key, "cut-run-") {
		t.Errorf("Key = %q, want a generated cut-run-* key", plan.Key)
	}
	if plan.Reused {
		t.Error("Reused = true, want false for a freshly created worktree")
	}
	if !plan.AutoRemove {
		t.Error("AutoRemove = false, want true — a temporary worktree is always cleaned up")
	}
	if f.wt.addDetachedBase != "origin/main" {
		t.Errorf("AddDetached base = %q, want the source", f.wt.addDetachedBase)
	}
	if f.wt.addBranch != "" {
		t.Error("Add() should not be called when no branch is given")
	}
}

func TestProvision_NoBranch_CurrentBranchError(t *testing.T) {
	t.Parallel()

	f := newFixture(t, &stubWorktrees{currentBranchErr: errors.New("no repo")}, true)

	if _, err := f.p.Provision(run.Spec{}); err == nil || !strings.Contains(err.Error(), "get current branch") {
		t.Fatalf("Provision() error = %v, want it to mention getting the current branch", err)
	}
}

func TestProvision_NoBranch_AddDetachedError(t *testing.T) {
	t.Parallel()

	f := newFixture(t, &stubWorktrees{currentBranch: "main", addDetachedErr: errors.New("git error")}, true)

	if _, err := f.p.Provision(run.Spec{}); err == nil || !strings.Contains(err.Error(), "create cutting") {
		t.Fatalf("Provision() error = %v, want the 'create cutting' prefix", err)
	}
}

// --- Provision: explicit branch ---

func TestProvision_Branch_CreatesWorktree(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name             string
		branchExists     bool
		source           string
		defaultBranch    string
		wantCreateBranch bool
		wantBase         string
	}{
		{name: "new branch from source", source: "main", wantCreateBranch: true, wantBase: "main"},
		{name: "new branch falls back to default_branch", defaultBranch: "develop", wantCreateBranch: true, wantBase: "develop"},
		{name: "source wins over default_branch", source: "main", defaultBranch: "develop", wantCreateBranch: true, wantBase: "main"},
		{name: "existing branch is not re-created", branchExists: true, wantCreateBranch: false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t, &stubWorktrees{addPath: "/tmp/ws", branchExists: c.branchExists}, true)

			plan, err := f.p.Provision(run.Spec{
				Branch:        "feature/foo",
				Source:        c.source,
				DefaultBranch: c.defaultBranch,
			})
			if err != nil {
				t.Fatalf("Provision() unexpected error: %v", err)
			}

			if f.wt.addCreateBranch != c.wantCreateBranch {
				t.Errorf("createBranch = %v, want %v", f.wt.addCreateBranch, c.wantCreateBranch)
			}
			if f.wt.addBase != c.wantBase {
				t.Errorf("base = %q, want %q", f.wt.addBase, c.wantBase)
			}
			if plan.Key != "feature/foo" || plan.EnvBranch != "feature/foo" {
				t.Errorf("plan = %+v, want the branch as key and env branch", plan)
			}
			if plan.Reused {
				t.Error("Reused = true, want false")
			}
		})
	}
}

func TestProvision_Branch_AddError(t *testing.T) {
	t.Parallel()

	f := newFixture(t, &stubWorktrees{addErr: errors.New("git error")}, true)

	if _, err := f.p.Provision(run.Spec{Branch: "feature/foo"}); err == nil || !strings.Contains(err.Error(), "create cutting") {
		t.Fatalf("Provision() error = %v, want the 'create cutting' prefix", err)
	}
}

// --- Provision: reusing an existing cutting ---

// A reused cutting is real work, not a temporary directory: it must not be
// swept up automatically unless the caller explicitly opted in.
func TestProvision_ReusedCutting_AutoRemovePolicy(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		removeAfter    bool
		wantAutoRemove bool
	}{
		{name: "without --remove-after it is left alone", removeAfter: false, wantAutoRemove: false},
		{name: "--remove-after opts into the temporary lifecycle", removeAfter: true, wantAutoRemove: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t, &stubWorktrees{exists: true, path: "/tmp/existing"}, true)

			plan, err := f.p.Provision(run.Spec{Branch: "feature/foo", RemoveAfter: c.removeAfter})
			if err != nil {
				t.Fatalf("Provision() unexpected error: %v", err)
			}

			if !plan.Reused {
				t.Error("Reused = false, want true")
			}
			if plan.Path != "/tmp/existing" {
				t.Errorf("Path = %q, want the existing cutting's path", plan.Path)
			}
			if plan.AutoRemove != c.wantAutoRemove {
				t.Errorf("AutoRemove = %v, want %v", plan.AutoRemove, c.wantAutoRemove)
			}
			if f.wt.addBranch != "" {
				t.Error("Add() should not be called for a cutting that already exists")
			}
			if gotLock := f.locks.acquireKey != ""; gotLock != c.wantAutoRemove {
				t.Errorf("lock acquired = %v, want %v (locks track only auto-removed cuttings)", gotLock, c.wantAutoRemove)
			}
		})
	}
}

// --- run locks ---

func TestProvision_AcquiresLockForTemporaryCutting(t *testing.T) {
	t.Parallel()

	f := newFixture(t, &stubWorktrees{currentBranch: "main", addDetachedPath: "/tmp/ws"}, true)

	plan, err := f.p.Provision(run.Spec{})
	if err != nil {
		t.Fatalf("Provision() unexpected error: %v", err)
	}
	if f.locks.acquireKey != plan.Key {
		t.Errorf("lock key = %q, want the plan key %q", f.locks.acquireKey, plan.Key)
	}
	if f.locks.acquirePath != "/tmp/ws" {
		t.Errorf("lock path = %q, want the worktree path", f.locks.acquirePath)
	}
}

func TestProvision_CleanupOnSignalDisabled_TakesNoLock(t *testing.T) {
	t.Parallel()

	f := newFixture(t, &stubWorktrees{currentBranch: "main", addDetachedPath: "/tmp/ws"}, false)

	if _, err := f.p.Provision(run.Spec{}); err != nil {
		t.Fatalf("Provision() unexpected error: %v", err)
	}
	if f.locks.acquireKey != "" {
		t.Error("a lock was acquired despite cleanup-on-signal being disabled")
	}
}

// Lock bookkeeping is best-effort: failing to record one must not abort a run.
func TestProvision_LockFailureIsOnlyAWarning(t *testing.T) {
	t.Parallel()

	f := newFixture(t, &stubWorktrees{currentBranch: "main", addDetachedPath: "/tmp/ws"}, true)
	f.locks.acquireErr = errors.New("disk full")

	plan, err := f.p.Provision(run.Spec{})
	if err != nil {
		t.Fatalf("Provision() error = %v, want the run to proceed", err)
	}
	if plan.Path != "/tmp/ws" {
		t.Error("the worktree should still be usable")
	}
	if !strings.Contains(f.errOut.String(), "warning: could not record run lock") {
		t.Errorf("stderr = %q, want the lock warning", f.errOut.String())
	}
}

// --- SweepOrphans ---

func TestSweepOrphans_ReportsCleanedKeys(t *testing.T) {
	t.Parallel()

	f := newFixture(t, &stubWorktrees{}, true)
	f.locks.sweepResult = []string{"cut-run-123"}

	f.p.SweepOrphans()

	if !strings.Contains(f.out.String(), "Cleaned up orphaned cutting from a previous run: cut-run-123") {
		t.Errorf("stdout = %q, want the swept key reported", f.out.String())
	}
}

func TestSweepOrphans_ErrorIsOnlyAWarning(t *testing.T) {
	t.Parallel()

	f := newFixture(t, &stubWorktrees{}, true)
	f.locks.sweepErr = errors.New("permission denied")

	f.p.SweepOrphans()

	if !strings.Contains(f.errOut.String(), "warning: orphan sweep failed") {
		t.Errorf("stderr = %q, want the sweep warning", f.errOut.String())
	}
}

func TestSweepOrphans_DisabledWhenCleanupOnSignalIsOff(t *testing.T) {
	t.Parallel()

	f := newFixture(t, &stubWorktrees{}, false)
	f.p.SweepOrphans()

	if f.locks.sweepCalled {
		t.Error("Sweep() was called despite cleanup-on-signal being disabled")
	}
}

// An orphan whose worktree is already gone is still a successful cleanup — the
// only thing left to remove is its lock file.
func TestSweepOrphans_RemoveCallback(t *testing.T) {
	t.Parallel()

	f := newFixture(t, &stubWorktrees{removeErrs: map[string]error{
		"already-gone": worktree.ErrWorktreeNotFound,
		"dirty":        errors.New("uncommitted changes"),
	}}, true)

	f.p.SweepOrphans()
	if f.locks.sweepRemove == nil {
		t.Fatal("Sweep() was not given a removal callback")
	}

	if err := f.locks.sweepRemove("already-gone"); err != nil {
		t.Errorf("removing an orphan whose worktree is gone = %v, want nil", err)
	}
	if err := f.locks.sweepRemove("dirty"); err == nil {
		t.Error("a real removal failure should be reported to the sweep")
	}
	if err := f.locks.sweepRemove("fine"); err != nil {
		t.Errorf("removing a normal orphan = %v, want nil", err)
	}
}

// --- Cleanup ---

func TestCleanup_RemovesWorktreeAndReleasesLock(t *testing.T) {
	t.Parallel()

	f := newFixture(t, &stubWorktrees{}, true)
	f.p.Cleanup(run.Plan{Key: "cut-run-1", AutoRemove: true})

	if len(f.wt.removed) != 1 || f.wt.removed[0] != "cut-run-1" {
		t.Errorf("removed = %v, want [cut-run-1]", f.wt.removed)
	}
	if f.locks.releaseKey != "cut-run-1" {
		t.Errorf("released lock = %q, want cut-run-1", f.locks.releaseKey)
	}
	if !strings.Contains(f.out.String(), "Cleaning up cutting...") {
		t.Errorf("stdout = %q, want the cleanup message", f.out.String())
	}
}

func TestCleanup_CleanupOnSignalDisabled_NoLockRelease(t *testing.T) {
	t.Parallel()

	f := newFixture(t, &stubWorktrees{}, false)
	f.p.Cleanup(run.Plan{Key: "cut-run-1", AutoRemove: true})

	if f.locks.releaseCalled {
		t.Error("Release() was called despite cleanup-on-signal being disabled")
	}
	if len(f.wt.removed) != 1 {
		t.Error("the worktree must still be removed — that cleanup is unconditional")
	}
}

// Cleanup failures are warnings: they must never mask the command's outcome.
func TestCleanup_FailuresAreWarnings(t *testing.T) {
	t.Parallel()

	f := newFixture(t, &stubWorktrees{removeErr: errors.New("worktree busy")}, true)
	f.locks.releaseErr = errors.New("lock file gone")

	f.p.Cleanup(run.Plan{Key: "cut-run-1", AutoRemove: true})

	stderr := f.errOut.String()
	if !strings.Contains(stderr, "warning: cleanup failed") {
		t.Errorf("stderr = %q, want the cleanup warning", stderr)
	}
	if !strings.Contains(stderr, "warning: could not remove run lock") {
		t.Errorf("stderr = %q, want the unlock warning", stderr)
	}
}

func TestRemoveReused(t *testing.T) {
	t.Parallel()

	f := newFixture(t, &stubWorktrees{}, true)
	f.p.RemoveReused("feature/foo")

	if len(f.wt.removed) != 1 || f.wt.removed[0] != "feature/foo" {
		t.Errorf("removed = %v, want [feature/foo]", f.wt.removed)
	}
	if f.locks.releaseCalled {
		t.Error("Release() should not be called — a reused cutting never took a lock")
	}
	if !strings.Contains(f.out.String(), `Removing cutting "feature/foo"...`) {
		t.Errorf("stdout = %q, want the removal message", f.out.String())
	}
}

// --- Invoke ---

type stubCommander struct {
	err error

	dir     string
	branch  string
	command []string
}

func (s *stubCommander) Run(_ context.Context, dir, branch string, command []string) error {
	s.dir, s.branch, s.command = dir, branch, command
	return s.err
}

func TestInvoke_PassesPlanToRunner(t *testing.T) {
	t.Parallel()

	c := &stubCommander{}
	plan := run.Plan{Path: "/tmp/ws", EnvBranch: "feature/foo"}

	outcome, err := run.Invoke(c, nil, plan, []string{"go", "test", "./..."})
	if err != nil {
		t.Fatalf("Invoke() unexpected error: %v", err)
	}
	if outcome.ExitCode != 0 || outcome.Interrupted {
		t.Errorf("outcome = %+v, want a clean run", outcome)
	}
	if c.dir != "/tmp/ws" || c.branch != "feature/foo" {
		t.Errorf("runner got dir=%q branch=%q, want the plan's values", c.dir, c.branch)
	}
	if strings.Join(c.command, " ") != "go test ./..." {
		t.Errorf("command = %v, want the args passed through", c.command)
	}
}

func TestInvoke_ExitErrorBecomesExitCode(t *testing.T) {
	t.Parallel()

	var exitErr *exec.ExitError
	if err := exec.Command("sh", "-c", "exit 7").Run(); !errors.As(err, &exitErr) {
		t.Skip("could not construct *exec.ExitError for test")
	}

	outcome, err := run.Invoke(&stubCommander{err: exitErr}, nil, run.Plan{}, []string{"sh"})
	if err != nil {
		t.Fatalf("Invoke() error = %v, want a non-zero exit reported via Outcome", err)
	}
	if outcome.ExitCode != 7 {
		t.Errorf("ExitCode = %d, want 7", outcome.ExitCode)
	}
	if outcome.Interrupted {
		t.Error("Interrupted = true, want false — the command exited on its own")
	}
}

// Anything that is not an exit status is a cuttings-level failure and must be
// returned, not converted into an exit code.
func TestInvoke_OtherErrorsAreReturned(t *testing.T) {
	t.Parallel()

	boom := errors.New("executable file not found")
	outcome, err := run.Invoke(&stubCommander{err: boom}, nil, run.Plan{}, []string{"nope"})
	if !errors.Is(err, boom) {
		t.Errorf("Invoke() error = %v, want %v", err, boom)
	}
	if outcome.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", outcome.ExitCode)
	}
}
