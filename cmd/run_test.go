/*
Copyright © 2026 Christoph Becker
*/
package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/ChristophBe/cuttings/internal/config"
)

// runFixture wires a run command to test doubles. Each fixture builds its own
// command and its own Deps, so no package-level state is touched and tests can
// run in parallel.
type runFixture struct {
	wt     *mockWorktreeManager
	runner *mockRunner
	locks  *mockLocks
	deps   *Deps
	stdout bytes.Buffer
	stderr bytes.Buffer
	stdin  io.Reader
}

func newRunFixture(wt *mockWorktreeManager, runner *mockRunner) *runFixture {
	// The lock store and the worktree manager share one call-order log, so a
	// test can assert that e.g. the worktree is created before it is locked.
	locks := &mockLocks{callOrder: &wt.callOrder}
	return &runFixture{
		wt:     wt,
		runner: runner,
		locks:  locks,
		// RunCleanupOnSignal defaults to true in real usage (config.Load sets it
		// via config.DefaultRunCleanupOnSignal); mirror that here so tests
		// exercise the enabled path unless one explicitly opts out.
		deps: &Deps{
			cfg:    &config.Config{RunCleanupOnSignal: true},
			wt:     wt,
			locks:  locks,
			runner: runner,
		},
		// Default to an already-exhausted reader so a test that unexpectedly
		// hits the removal prompt gets a deterministic "no" (EOF) instead of
		// blocking.
		stdin: strings.NewReader(""),
	}
}

// exec runs the command with args exactly as the CLI would: real flag parsing,
// real "--" handling and the real Args validator, none of which were exercised
// when tests called RunE directly.
func (f *runFixture) exec(args ...string) error {
	cmd := newRunCmd(f.deps)
	cmd.SetOut(&f.stdout)
	cmd.SetErr(&f.stderr)
	cmd.SetIn(f.stdin)
	cmd.SetArgs(args)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	return cmd.Execute()
}

// --- tests ---

// --- no-branch (detached HEAD) path ---

func TestRunCmd_NoBranch_UsesDetachedWorktree(t *testing.T) {
	t.Parallel()

	wt := &mockWorktreeManager{currentBranch: "feature/foo", addDetachedPath: "/tmp/ws"}
	runner := &mockRunner{}
	f := newRunFixture(wt, runner)

	if err := f.exec("--", "true"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if wt.addBranch != "" {
		t.Error("Add() should not be called when no --branch is set")
	}
	if wt.addDetachedName == "" {
		t.Error("AddDetached() was not called")
	}
	if !strings.HasPrefix(wt.addDetachedName, "cut-run-") {
		t.Errorf("detached worktree name = %q, want prefix %q", wt.addDetachedName, "cut-run-")
	}
}

func TestRunCmd_NoBranch_EnvBranchIsCurrentBranch(t *testing.T) {
	t.Parallel()

	wt := &mockWorktreeManager{currentBranch: "feature/foo", addDetachedPath: "/tmp/ws"}
	runner := &mockRunner{}
	f := newRunFixture(wt, runner)

	if err := f.exec("--", "true"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if runner.runBranch != "feature/foo" {
		t.Errorf("CUTTING_BRANCH = %q, want %q", runner.runBranch, "feature/foo")
	}
}

func TestRunCmd_NoBranch_FromFlagPassedToAddDetached(t *testing.T) {
	t.Parallel()

	wt := &mockWorktreeManager{currentBranch: "main", addDetachedPath: "/tmp/ws"}
	f := newRunFixture(wt, &mockRunner{})

	if err := f.exec("--source", "origin/main", "--", "true"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if wt.addDetachedBase != "origin/main" {
		t.Errorf("AddDetached base = %q, want %q", wt.addDetachedBase, "origin/main")
	}
}

func TestRunCmd_NoBranch_CurrentBranchError(t *testing.T) {
	t.Parallel()

	wt := &mockWorktreeManager{currentBranchErr: errors.New("no git repo")}
	f := newRunFixture(wt, &mockRunner{})

	err := f.exec("--", "true")
	if err == nil {
		t.Fatal("expected error when CurrentBranch() fails, got nil")
	}
	if !strings.Contains(err.Error(), "get current branch") {
		t.Errorf("error %q missing expected prefix", err.Error())
	}
}

func TestRunCmd_NoBranch_AddDetachedFails(t *testing.T) {
	t.Parallel()

	wt := &mockWorktreeManager{currentBranch: "main", addDetachedErr: errors.New("git error")}
	f := newRunFixture(wt, &mockRunner{})

	err := f.exec("--", "true")
	if err == nil {
		t.Fatal("expected error when AddDetached() fails, got nil")
	}
	if !strings.Contains(err.Error(), "create cutting") {
		t.Errorf("error %q missing 'create cutting' prefix", err.Error())
	}
}

func TestRunCmd_NoBranch_CleanupCalled(t *testing.T) {
	t.Parallel()

	wt := &mockWorktreeManager{currentBranch: "main", addDetachedPath: "/tmp/ws"}
	f := newRunFixture(wt, &mockRunner{})

	if err := f.exec("--", "true"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !wt.removeCalled {
		t.Error("Remove() was not called after successful run")
	}
	// The worktree key should be the generated cut-run-* name, not the branch name.
	if !strings.HasPrefix(wt.removeKey, "cut-run-") {
		t.Errorf("Remove key = %q, want prefix %q", wt.removeKey, "cut-run-")
	}
}

// --- explicit --branch path ---

func TestRunCmd_ExistingBranch_RunsInPlace_NoCreate(t *testing.T) {
	t.Parallel()

	wt := &mockWorktreeManager{existsResult: true, pathResult: "/tmp/existing-ws"}
	runner := &mockRunner{}
	f := newRunFixture(wt, runner)

	f.stdin = strings.NewReader("n\n")

	if err := f.exec("feature/exists", "--", "echo", "hello"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wt.addBranch != "" {
		t.Error("Add() should not have been called for an existing cutting")
	}
	if runner.runDir != "/tmp/existing-ws" {
		t.Errorf("runner dir = %q, want %q", runner.runDir, "/tmp/existing-ws")
	}
	if runner.runBranch != "feature/exists" {
		t.Errorf("runner branch = %q, want %q", runner.runBranch, "feature/exists")
	}
}

func TestRunCmd_ExistingBranch_PromptRemove_Yes(t *testing.T) {
	t.Parallel()

	wt := &mockWorktreeManager{existsResult: true, pathResult: "/tmp/existing-ws"}
	f := newRunFixture(wt, &mockRunner{})

	f.stdin = strings.NewReader("y\n")

	if err := f.exec("feature/exists", "--", "echo", "hello"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	stdout := f.stdout.String()

	if !wt.removeCalled {
		t.Error("expected Remove() to be called after confirming removal")
	}
	if !strings.Contains(stdout, `Removing cutting "feature/exists"`) {
		t.Errorf("stdout = %q, want removal confirmation message", stdout)
	}
}

func TestRunCmd_ExistingBranch_PromptRemove_Yes_RemoveFails(t *testing.T) {
	t.Parallel()

	wt := &mockWorktreeManager{existsResult: true, pathResult: "/tmp/existing-ws", removeErr: errors.New("remove failed")}
	f := newRunFixture(wt, &mockRunner{})

	f.stdin = strings.NewReader("y\n")

	if err := f.exec("feature/exists", "--", "echo", "hello"); err != nil {
		t.Fatalf("a Remove() failure after confirming removal should be a non-fatal warning, got error: %v", err)
	}
	if !wt.removeCalled {
		t.Error("expected Remove() to still be attempted")
	}
}

func TestRunCmd_ExistingBranch_PromptRemove_No(t *testing.T) {
	t.Parallel()

	wt := &mockWorktreeManager{existsResult: true, pathResult: "/tmp/existing-ws"}
	f := newRunFixture(wt, &mockRunner{})

	f.stdin = strings.NewReader("n\n")

	if err := f.exec("feature/exists", "--", "echo", "hello"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	stdout := f.stdout.String()

	if wt.removeCalled {
		t.Error("Remove() should not have been called after declining removal")
	}
	if !strings.Contains(stdout, `Leaving cutting "feature/exists" in place`) {
		t.Errorf("stdout = %q, want the kept-in-place message", stdout)
	}
}

func TestRunCmd_ExistingBranch_PromptDefaultsToNoOnEOF(t *testing.T) {
	t.Parallel()

	wt := &mockWorktreeManager{existsResult: true, pathResult: "/tmp/existing-ws"}
	f := newRunFixture(wt, &mockRunner{})

	f.stdin = strings.NewReader("") // immediate EOF, e.g. no terminal attached

	if err := f.exec("feature/exists", "--", "echo", "hello"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wt.removeCalled {
		t.Error("Remove() should not have been called when the prompt hits EOF")
	}
}

func TestRunCmd_ExistingBranch_RemoveAfterFlag_SkipsPromptAndRemoves(t *testing.T) {
	t.Parallel()

	wt := &mockWorktreeManager{existsResult: true, pathResult: "/tmp/existing-ws"}
	f := newRunFixture(wt, &mockRunner{})

	// No reader input at all — --remove-after must never read from it.
	f.stdin = strings.NewReader("")

	if err := f.exec("--remove-after", "feature/exists", "--", "echo", "hello"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !wt.removeCalled {
		t.Error("expected Remove() to be called with --remove-after set")
	}
	if wt.removeKey != "feature/exists" {
		t.Errorf("Remove key = %q, want %q", wt.removeKey, "feature/exists")
	}
}

func TestRunCmd_ExistingBranch_RemoveAfterFlag_LocksLikeTemporary(t *testing.T) {
	t.Parallel()

	wt := &mockWorktreeManager{existsResult: true, pathResult: "/tmp/existing-ws"}
	f := newRunFixture(wt, &mockRunner{})

	if err := f.exec("--remove-after", "feature/exists", "--", "echo", "hello"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !f.locks.acquireCalled {
		t.Error("expected Lock() to be called when --remove-after opts a reused cutting into the temporary lifecycle")
	}
	if !f.locks.releaseCalled {
		t.Error("expected Unlock() to be called after cleanup")
	}
}

func TestRunCmd_ExistingBranch_WithoutRemoveAfter_NoLockRegistered(t *testing.T) {
	t.Parallel()

	wt := &mockWorktreeManager{existsResult: true, pathResult: "/tmp/existing-ws"}
	f := newRunFixture(wt, &mockRunner{})

	f.stdin = strings.NewReader("n\n")

	if err := f.exec("feature/exists", "--", "echo", "hello"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.locks.acquireCalled {
		t.Error("Lock() should not be called for a reused cutting without --remove-after")
	}
	if f.locks.releaseCalled {
		t.Error("Unlock() should not be called for a reused cutting without --remove-after")
	}
}

// Note: the "interrupted by a real OS signal" case for a reused cutting
// (no prompt, no removal) is covered at the e2e level in e2e/run_test.go —
// run.go's sigCh is only ever fed by signal.Notify, the same reason the
// existing signal-handling tests above test cancellation semantics via
// signalAwareRun directly rather than delivering a real signal here.

func TestRunCmd_AddFails(t *testing.T) {
	t.Parallel()

	wt := &mockWorktreeManager{addErr: errors.New("git error")}
	f := newRunFixture(wt, &mockRunner{})

	err := f.exec("feature/new", "--", "echo", "hello")
	if err == nil {
		t.Fatal("expected error when Add() fails, got nil")
	}
	if !strings.Contains(err.Error(), "create cutting") {
		t.Errorf("error %q missing 'create cutting' prefix", err.Error())
	}
}

func TestRunCmd_Success_CleanupCalled(t *testing.T) {
	t.Parallel()

	wt := &mockWorktreeManager{addPath: "/tmp/ws"}
	runner := &mockRunner{}
	f := newRunFixture(wt, runner)

	if err := f.exec("feature/foo", "--", "echo", "hello"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !wt.removeCalled {
		t.Error("Remove() was not called after successful run")
	}
	if runner.runDir != "/tmp/ws" {
		t.Errorf("runner dir = %q, want %q", runner.runDir, "/tmp/ws")
	}
}

func TestRunCmd_CommandError_CleanupCalled(t *testing.T) {
	t.Parallel()

	wt := &mockWorktreeManager{addPath: "/tmp/ws"}
	runner := &mockRunner{runErr: errors.New("command failed")}
	f := newRunFixture(wt, runner)

	err := f.exec("feature/foo", "--", "failing-cmd")
	if err == nil {
		t.Fatal("expected error from failing command, got nil")
	}
	if !wt.removeCalled {
		t.Error("Remove() was not called after command error")
	}
}

// A command that exits non-zero must not look like a cuttings failure: the
// cutting is still cleaned up, and the code travels back as an ExitCodeError
// for Execute to turn into the process's own exit status.
func TestRunCmd_ExitError_CleanupCalledAndExitCodePropagated(t *testing.T) {
	t.Parallel()

	wt := &mockWorktreeManager{addPath: "/tmp/ws"}

	var exitErr *exec.ExitError
	if err := exec.Command("sh", "-c", "exit 3").Run(); !errors.As(err, &exitErr) {
		t.Skip("could not construct *exec.ExitError for test")
	}

	f := newRunFixture(wt, &mockRunner{runErr: exitErr})

	err := f.exec("feature/foo", "--", "sh", "-c", "exit 3")

	var codeErr *ExitCodeError
	if !errors.As(err, &codeErr) {
		t.Fatalf("error = %v, want an *ExitCodeError", err)
	}
	if codeErr.Code != 3 {
		t.Errorf("exit code = %d, want 3", codeErr.Code)
	}
	if !wt.removeCalled {
		t.Error("Remove() was not called — cleanup must still happen on a non-zero exit")
	}
}

// Cleanup runs on the way out of RunE, so its output is already flushed by the
// time the exit code reaches Execute.
func TestRunCmd_ExitCode_CleanupHappensBeforeReturn(t *testing.T) {
	t.Parallel()

	wt := &mockWorktreeManager{addPath: "/tmp/ws"}

	var exitErr *exec.ExitError
	if err := exec.Command("sh", "-c", "exit 3").Run(); !errors.As(err, &exitErr) {
		t.Skip("could not construct *exec.ExitError for test")
	}

	f := newRunFixture(wt, &mockRunner{runErr: exitErr})
	_ = f.exec("feature/foo", "--", "sh", "-c", "exit 3")

	if !strings.Contains(f.stdout.String(), "Cleaning up cutting") {
		t.Errorf("stdout = %q, want the cleanup message before the exit code propagates", f.stdout.String())
	}
}

func TestRunCmd_ExplicitBranch(t *testing.T) {
	t.Parallel()

	wt := &mockWorktreeManager{addPath: "/tmp/ws"}
	f := newRunFixture(wt, &mockRunner{})

	if err := f.exec("feature/my-branch", "--", "true"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if wt.addBranch != "feature/my-branch" {
		t.Errorf("branch = %q, want %q", wt.addBranch, "feature/my-branch")
	}
}

func TestRunCmd_FromFlagPassedToAdd(t *testing.T) {
	t.Parallel()

	wt := &mockWorktreeManager{addPath: "/tmp/ws"}
	f := newRunFixture(wt, &mockRunner{})

	if err := f.exec("--source", "main", "feature/new", "--", "true"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if wt.addBase != "main" {
		t.Errorf("Add base = %q, want %q", wt.addBase, "main")
	}
}

func TestRunCmd_BranchExists_NoCreate(t *testing.T) {
	t.Parallel()

	wt := &mockWorktreeManager{addPath: "/tmp/ws", branchExists: true}
	f := newRunFixture(wt, &mockRunner{})

	if err := f.exec("existing-branch", "--", "true"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if wt.addCreateBranch {
		t.Error("Add() called with createBranch=true for an existing branch")
	}
}

func TestRunCmd_BranchNotExists_Create(t *testing.T) {
	t.Parallel()

	wt := &mockWorktreeManager{addPath: "/tmp/ws", branchExists: false}
	f := newRunFixture(wt, &mockRunner{})

	if err := f.exec("new-branch", "--", "true"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !wt.addCreateBranch {
		t.Error("Add() called with createBranch=false for a non-existing branch")
	}
}

func TestRunCmd_RunnerReceivesCorrectArgs(t *testing.T) {
	t.Parallel()

	wt := &mockWorktreeManager{addPath: "/tmp/testws"}
	runner := &mockRunner{}
	f := newRunFixture(wt, runner)

	if err := f.exec("my-branch", "--", "go", "test", "./..."); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if runner.runBranch != "my-branch" {
		t.Errorf("runner branch = %q, want %q", runner.runBranch, "my-branch")
	}
	wantCmd := []string{"go", "test", "./..."}
	if fmt.Sprint(runner.runCommand) != fmt.Sprint(wantCmd) {
		t.Errorf("runner command = %v, want %v", runner.runCommand, wantCmd)
	}
}

func TestRunCmd_DefaultBranchUsedAsFromBase(t *testing.T) {
	t.Parallel()

	wt := &mockWorktreeManager{addPath: "/tmp/ws"}
	f := newRunFixture(wt, &mockRunner{})

	f.deps.cfg = &config.Config{DefaultBranch: "develop", RunCleanupOnSignal: true}

	if err := f.exec("feature/new", "--", "true"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if wt.addBase != "develop" {
		t.Errorf("Add base = %q, want %q (config DefaultBranch)", wt.addBase, "develop")
	}
}

// --- orphan sweep ---

func TestRunCmd_SweepOrphans_CalledBeforeCreate(t *testing.T) {
	t.Parallel()

	wt := &mockWorktreeManager{addDetachedPath: "/tmp/ws"}
	f := newRunFixture(wt, &mockRunner{})

	if err := f.exec("--", "true"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(wt.callOrder) == 0 || wt.callOrder[0] != "SweepOrphans" {
		t.Errorf("call order = %v, want SweepOrphans first", wt.callOrder)
	}
}

func TestRunCmd_SweepOrphans_PrintsCleanedKeys(t *testing.T) {
	t.Parallel()

	wt := &mockWorktreeManager{addDetachedPath: "/tmp/ws"}
	f := newRunFixture(wt, &mockRunner{})
	f.locks.sweepResult = []string{"cut-run-123"}

	if err := f.exec("--", "true"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	stdout := f.stdout.String()

	if !strings.Contains(stdout, "Cleaned up orphaned cutting from a previous run: cut-run-123") {
		t.Errorf("stdout = %q, want it to mention the cleaned-up orphan", stdout)
	}
}

func TestRunCmd_SweepOrphans_ErrorIsNonFatal(t *testing.T) {
	t.Parallel()

	wt := &mockWorktreeManager{addDetachedPath: "/tmp/ws"}
	f := newRunFixture(wt, &mockRunner{})
	f.locks.sweepErr = errors.New("sweep failed")

	if err := f.exec("--", "true"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wt.addDetachedName == "" {
		t.Error("run should still proceed to create a worktree despite a sweep error")
	}
}

// --- run lock / unlock ---

func TestRunCmd_LockCalledAfterWorktreeCreated_UnlockCalledOnCleanup(t *testing.T) {
	t.Parallel()

	wt := &mockWorktreeManager{addDetachedPath: "/tmp/ws"}
	f := newRunFixture(wt, &mockRunner{})

	if err := f.exec("--", "true"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !f.locks.acquireCalled {
		t.Error("Lock() was not called")
	}
	if !strings.HasPrefix(f.locks.acquireKey, "cut-run-") {
		t.Errorf("Lock key = %q, want prefix %q", f.locks.acquireKey, "cut-run-")
	}
	if !f.locks.releaseCalled {
		t.Error("Unlock() was not called")
	}
	if f.locks.releaseKey != f.locks.acquireKey {
		t.Errorf("Unlock key = %q, want it to match Lock key %q", f.locks.releaseKey, f.locks.acquireKey)
	}

	// Lock must happen after the worktree is created (AddDetached), and
	// Unlock must happen as part of cleanup (after Remove, or at least after
	// the command finished — call order only guarantees relative ordering of
	// recorded operations, so check indices directly).
	lockIdx, addIdx, unlockIdx := -1, -1, -1
	for i, c := range wt.callOrder {
		switch c {
		case "AddDetached":
			addIdx = i
		case "Lock":
			lockIdx = i
		case "Unlock":
			unlockIdx = i
		}
	}
	if addIdx >= lockIdx || lockIdx >= unlockIdx {
		t.Errorf("call order = %v, want AddDetached before Lock before Unlock", wt.callOrder)
	}
}

func TestRunCmd_LockFails_RunStillProceeds(t *testing.T) {
	t.Parallel()

	wt := &mockWorktreeManager{addDetachedPath: "/tmp/ws"}
	runner := &mockRunner{}
	f := newRunFixture(wt, runner)
	f.locks.acquireErr = errors.New("lock failed")

	if err := f.exec("--", "true"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runner.runDir != "/tmp/ws" {
		t.Error("command was not run despite Lock() failing")
	}
	if !wt.removeCalled {
		t.Error("Remove() was not called despite Lock() failing")
	}
}

func TestRunCmd_UnlockFails_CleanupStillReportsSuccess(t *testing.T) {
	t.Parallel()

	wt := &mockWorktreeManager{addDetachedPath: "/tmp/ws"}
	f := newRunFixture(wt, &mockRunner{})
	f.locks.releaseErr = errors.New("unlock failed")

	if err := f.exec("--", "true"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !wt.removeCalled {
		t.Error("Remove() was not called")
	}
}

// --- run_cleanup_on_signal = false ---

func TestRunCmd_CleanupOnSignalDisabled_SkipsSweepLockAndUnlock(t *testing.T) {
	t.Parallel()

	wt := &mockWorktreeManager{addDetachedPath: "/tmp/ws"}
	f := newRunFixture(wt, &mockRunner{})

	f.deps.cfg.RunCleanupOnSignal = false

	if err := f.exec("--", "true"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if f.locks.sweepCalled {
		t.Error("SweepOrphans() was called despite run_cleanup_on_signal=false")
	}
	if f.locks.acquireCalled {
		t.Error("Lock() was called despite run_cleanup_on_signal=false")
	}
	if f.locks.releaseCalled {
		t.Error("Unlock() was called despite run_cleanup_on_signal=false")
	}
	// Remove() is unconditional — the plain defer-based cleanup this feature
	// was layered on top of must still run regardless of the config toggle.
	if !wt.removeCalled {
		t.Error("Remove() was not called")
	}
}

func TestRunCmd_CleanupOnSignalDisabled_SignalDoesNotCancelRunningCommand(t *testing.T) {
	t.Parallel()

	wt := &mockWorktreeManager{addDetachedPath: "/tmp/ws"}
	started := make(chan struct{})
	finished := make(chan struct{})
	runner := &mockRunner{runFunc: func(ctx context.Context) error {
		close(started)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-finished:
			return nil
		}
	}}
	f := newRunFixture(wt, runner)

	f.deps.cfg.RunCleanupOnSignal = false

	done := make(chan error, 1)
	go func() { done <- f.exec("--", "true") }()

	<-started
	// With the feature disabled, nothing is listening on sigCh, so this must
	// have no effect on the running command — simulate that directly by
	// confirming the command only finishes when we close(finished), not on
	// some external cancellation.
	select {
	case <-done:
		t.Fatal("run returned before the command finished — signal handling should be disabled")
	case <-time.After(50 * time.Millisecond):
	}
	close(finished)

	if err := <-done; err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- argument validation and the deprecated --branch flag ---
//
// These paths were unreachable while tests invoked RunE directly: the Args
// validator and ArgsLenAtDash only run during real Cobra flag parsing.

func TestRunCmd_ArgsValidation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "no command at all",
			args:    []string{},
			wantErr: `requires a command to run after "--"`,
		},
		{
			name:    "branch given but no command after --",
			args:    []string{"feature/foo", "--"},
			wantErr: `requires a command to run after "--"`,
		},
		{
			name:    "more than one argument before --",
			args:    []string{"feature/foo", "extra", "--", "true"},
			wantErr: `accepts at most 1 branch argument before "--"`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			f := newRunFixture(&mockWorktreeManager{addDetachedPath: "/tmp/ws"}, &mockRunner{})

			err := f.exec(c.args...)
			if err == nil {
				t.Fatalf("args %v: expected a validation error", c.args)
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err, c.wantErr)
			}
			if f.wt.addDetachedName != "" || f.wt.addBranch != "" {
				t.Error("no worktree should be created when argument validation fails")
			}
		})
	}
}

// A command may be run without "--" as long as nothing looks like a flag;
// the branch is then not set, so this takes the detached-HEAD path.
func TestRunCmd_NoDashDash_TreatsAllArgsAsTheCommand(t *testing.T) {
	t.Parallel()

	wt := &mockWorktreeManager{currentBranch: "main", addDetachedPath: "/tmp/ws"}
	runner := &mockRunner{}
	f := newRunFixture(wt, runner)

	if err := f.exec("true"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wt.addDetachedName == "" {
		t.Error("expected the detached-HEAD path when no branch precedes --")
	}
	if len(runner.runCommand) != 1 || runner.runCommand[0] != "true" {
		t.Errorf("command = %v, want [true]", runner.runCommand)
	}
}

// The deprecated --branch flag still selects a branch, for compatibility.
func TestRunCmd_DeprecatedBranchFlag(t *testing.T) {
	t.Parallel()

	wt := &mockWorktreeManager{addPath: "/tmp/ws"}
	f := newRunFixture(wt, &mockRunner{})

	if err := f.exec("--branch", "feature/foo", "--", "true"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wt.addBranch != "feature/foo" {
		t.Errorf("Add branch = %q, want %q", wt.addBranch, "feature/foo")
	}
}

func TestRunCmd_PositionalAndDeprecatedFlagConflict(t *testing.T) {
	t.Parallel()

	f := newRunFixture(&mockWorktreeManager{addPath: "/tmp/ws"}, &mockRunner{})

	err := f.exec("--branch", "feature/a", "feature/b", "--", "true")
	if err == nil {
		t.Fatal("expected an error when combining --branch with a positional branch")
	}
	if !strings.Contains(err.Error(), "cannot combine") {
		t.Errorf("error = %q, want the 'cannot combine' message", err)
	}
	if f.wt.addBranch != "" {
		t.Error("no worktree should be created when the two branch inputs conflict")
	}
}

// Warnings go to stderr, progress to stdout — the split e2e assertions and
// shell redirection both rely on.
func TestRunCmd_WarningsGoToStderr(t *testing.T) {
	t.Parallel()

	wt := &mockWorktreeManager{addDetachedPath: "/tmp/ws", removeErr: errors.New("cleanup boom")}
	f := newRunFixture(wt, &mockRunner{})

	if err := f.exec("--", "true"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(f.stderr.String(), "warning: cleanup failed") {
		t.Errorf("stderr = %q, want the cleanup warning", f.stderr.String())
	}
	if strings.Contains(f.stdout.String(), "warning:") {
		t.Errorf("stdout should not carry warnings:\n%s", f.stdout.String())
	}
}
