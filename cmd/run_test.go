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
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ChristophBe/cuttings/internal/config"
	"github.com/ChristophBe/cuttings/internal/worktree"
)

// --- mock implementations ---

type mockWorktreeManager struct {
	existsResult     bool
	branchExists     bool
	addPath          string
	addErr           error
	addDetachedPath  string
	addDetachedErr   error
	pathResult       string
	currentBranch    string
	currentBranchErr error
	removeErr        error
	lockErr          error
	unlockErr        error
	sweepResult      []string
	sweepErr         error

	// recorded call arguments
	addBranch       string
	addCreateBranch bool
	addBase         string
	addDetachedName string
	addDetachedBase string
	removeCalled    bool
	removeKey       string
	lockCalled      bool
	lockKey         string
	unlockCalled    bool
	unlockKey       string
	sweepCalled     bool
	// callOrder records the order in which the operations below were invoked,
	// so tests can assert e.g. that sweep happens before worktree creation.
	callOrder []string
}

func (m *mockWorktreeManager) Exists(_ string) bool       { return m.existsResult }
func (m *mockWorktreeManager) BranchExists(_ string) bool { return m.branchExists }
func (m *mockWorktreeManager) Add(branch string, createBranch bool, base string) (string, error) {
	m.callOrder = append(m.callOrder, "Add")
	m.addBranch = branch
	m.addCreateBranch = createBranch
	m.addBase = base
	return m.addPath, m.addErr
}
func (m *mockWorktreeManager) AddDetached(name, base string) (string, error) {
	m.callOrder = append(m.callOrder, "AddDetached")
	m.addDetachedName = name
	m.addDetachedBase = base
	return m.addDetachedPath, m.addDetachedErr
}
func (m *mockWorktreeManager) CurrentBranch() (string, error) {
	return m.currentBranch, m.currentBranchErr
}
func (m *mockWorktreeManager) Remove(key string, _ bool) error {
	m.callOrder = append(m.callOrder, "Remove")
	m.removeCalled = true
	m.removeKey = key
	return m.removeErr
}
func (m *mockWorktreeManager) ListBranches() ([]string, error)    { return nil, nil }
func (m *mockWorktreeManager) List() ([]worktree.Worktree, error) { return nil, nil }
func (m *mockWorktreeManager) Path(_ string) string               { return m.pathResult }
func (m *mockWorktreeManager) Lock(key string) error {
	m.callOrder = append(m.callOrder, "Lock")
	m.lockCalled = true
	m.lockKey = key
	return m.lockErr
}
func (m *mockWorktreeManager) Unlock(key string) error {
	m.callOrder = append(m.callOrder, "Unlock")
	m.unlockCalled = true
	m.unlockKey = key
	return m.unlockErr
}
func (m *mockWorktreeManager) SweepOrphans() ([]string, error) {
	m.callOrder = append(m.callOrder, "SweepOrphans")
	m.sweepCalled = true
	return m.sweepResult, m.sweepErr
}

type mockRunner struct {
	runErr error
	// runFunc, if set, is invoked instead of returning runErr directly — used
	// by tests that need to observe or react to ctx (e.g. block until it is
	// canceled to simulate a signal arriving mid-run).
	runFunc func(ctx context.Context) error

	// recorded call arguments
	runDir     string
	runBranch  string
	runCommand []string
}

func (m *mockRunner) Run(ctx context.Context, dir, branch string, command []string) error {
	m.runDir = dir
	m.runBranch = branch
	m.runCommand = command
	if m.runFunc != nil {
		return m.runFunc(ctx)
	}
	return m.runErr
}

// setupRunTest replaces global deps and flags with test doubles and returns a
// restore function that must be deferred by the caller.
func setupRunTest(wt *mockWorktreeManager, runner *mockRunner) func() {
	savedDeps := deps
	savedRunBranch := runBranch
	savedRunSource := runSource
	savedRunRemoveAfter := runRemoveAfter
	savedExitFn := exitFn
	savedPromptReader := promptReader

	// RunCleanupOnSignal defaults to true in real usage (config.Load sets it via
	// config.DefaultRunCleanupOnSignal); mirror that here so existing tests keep
	// exercising the enabled path unless a test explicitly opts out.
	deps.cfg = &config.Config{RunCleanupOnSignal: true}
	deps.wt = wt
	deps.runner = runner

	runBranch = ""
	runSource = ""
	runRemoveAfter = false
	// Default to an already-exhausted reader so a test that unexpectedly hits
	// the removal prompt gets a deterministic "no" (EOF) instead of blocking.
	promptReader = strings.NewReader("")

	return func() {
		deps = savedDeps
		runBranch = savedRunBranch
		runSource = savedRunSource
		runRemoveAfter = savedRunRemoveAfter
		exitFn = savedExitFn
		promptReader = savedPromptReader
	}
}

// callRunE invokes the run command's RunE handler directly with the given args.
func callRunE(args []string) error {
	return runCmd.RunE(runCmd, args)
}

// --- tests ---

// --- no-branch (detached HEAD) path ---

func TestRunCmd_NoBranch_UsesDetachedWorktree(t *testing.T) {
	wt := &mockWorktreeManager{currentBranch: "feature/foo", addDetachedPath: "/tmp/ws"}
	runner := &mockRunner{}
	restore := setupRunTest(wt, runner)
	defer restore()

	if err := callRunE([]string{"true"}); err != nil {
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
	wt := &mockWorktreeManager{currentBranch: "feature/foo", addDetachedPath: "/tmp/ws"}
	runner := &mockRunner{}
	restore := setupRunTest(wt, runner)
	defer restore()

	if err := callRunE([]string{"true"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if runner.runBranch != "feature/foo" {
		t.Errorf("CUTTING_BRANCH = %q, want %q", runner.runBranch, "feature/foo")
	}
}

func TestRunCmd_NoBranch_FromFlagPassedToAddDetached(t *testing.T) {
	wt := &mockWorktreeManager{currentBranch: "main", addDetachedPath: "/tmp/ws"}
	restore := setupRunTest(wt, &mockRunner{})
	defer restore()

	runSource = "origin/main"

	if err := callRunE([]string{"true"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if wt.addDetachedBase != "origin/main" {
		t.Errorf("AddDetached base = %q, want %q", wt.addDetachedBase, "origin/main")
	}
}

func TestRunCmd_NoBranch_CurrentBranchError(t *testing.T) {
	wt := &mockWorktreeManager{currentBranchErr: errors.New("no git repo")}
	restore := setupRunTest(wt, &mockRunner{})
	defer restore()

	err := callRunE([]string{"true"})
	if err == nil {
		t.Fatal("expected error when CurrentBranch() fails, got nil")
	}
	if !strings.Contains(err.Error(), "get current branch") {
		t.Errorf("error %q missing expected prefix", err.Error())
	}
}

func TestRunCmd_NoBranch_AddDetachedFails(t *testing.T) {
	wt := &mockWorktreeManager{currentBranch: "main", addDetachedErr: errors.New("git error")}
	restore := setupRunTest(wt, &mockRunner{})
	defer restore()

	err := callRunE([]string{"true"})
	if err == nil {
		t.Fatal("expected error when AddDetached() fails, got nil")
	}
	if !strings.Contains(err.Error(), "create cutting") {
		t.Errorf("error %q missing 'create cutting' prefix", err.Error())
	}
}

func TestRunCmd_NoBranch_CleanupCalled(t *testing.T) {
	wt := &mockWorktreeManager{currentBranch: "main", addDetachedPath: "/tmp/ws"}
	restore := setupRunTest(wt, &mockRunner{})
	defer restore()

	if err := callRunE([]string{"true"}); err != nil {
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
	wt := &mockWorktreeManager{existsResult: true, pathResult: "/tmp/existing-ws"}
	runner := &mockRunner{}
	restore := setupRunTest(wt, runner)
	defer restore()

	runBranch = "feature/exists"
	promptReader = strings.NewReader("n\n")

	if err := callRunE([]string{"echo", "hello"}); err != nil {
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
	wt := &mockWorktreeManager{existsResult: true, pathResult: "/tmp/existing-ws"}
	restore := setupRunTest(wt, &mockRunner{})
	defer restore()

	runBranch = "feature/exists"
	promptReader = strings.NewReader("y\n")

	stdout := captureStdout(t, func() {
		if err := callRunE([]string{"echo", "hello"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !wt.removeCalled {
		t.Error("expected Remove() to be called after confirming removal")
	}
	if !strings.Contains(stdout, `Removing cutting "feature/exists"`) {
		t.Errorf("stdout = %q, want removal confirmation message", stdout)
	}
}

func TestRunCmd_ExistingBranch_PromptRemove_Yes_RemoveFails(t *testing.T) {
	wt := &mockWorktreeManager{existsResult: true, pathResult: "/tmp/existing-ws", removeErr: errors.New("remove failed")}
	restore := setupRunTest(wt, &mockRunner{})
	defer restore()

	runBranch = "feature/exists"
	promptReader = strings.NewReader("y\n")

	if err := callRunE([]string{"echo", "hello"}); err != nil {
		t.Fatalf("a Remove() failure after confirming removal should be a non-fatal warning, got error: %v", err)
	}
	if !wt.removeCalled {
		t.Error("expected Remove() to still be attempted")
	}
}

func TestRunCmd_ExistingBranch_PromptRemove_No(t *testing.T) {
	wt := &mockWorktreeManager{existsResult: true, pathResult: "/tmp/existing-ws"}
	restore := setupRunTest(wt, &mockRunner{})
	defer restore()

	runBranch = "feature/exists"
	promptReader = strings.NewReader("n\n")

	stdout := captureStdout(t, func() {
		if err := callRunE([]string{"echo", "hello"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if wt.removeCalled {
		t.Error("Remove() should not have been called after declining removal")
	}
	if !strings.Contains(stdout, `Leaving cutting "feature/exists" in place`) {
		t.Errorf("stdout = %q, want the kept-in-place message", stdout)
	}
}

func TestRunCmd_ExistingBranch_PromptDefaultsToNoOnEOF(t *testing.T) {
	wt := &mockWorktreeManager{existsResult: true, pathResult: "/tmp/existing-ws"}
	restore := setupRunTest(wt, &mockRunner{})
	defer restore()

	runBranch = "feature/exists"
	promptReader = strings.NewReader("") // immediate EOF, e.g. no terminal attached

	if err := callRunE([]string{"echo", "hello"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wt.removeCalled {
		t.Error("Remove() should not have been called when the prompt hits EOF")
	}
}

func TestRunCmd_ExistingBranch_RemoveAfterFlag_SkipsPromptAndRemoves(t *testing.T) {
	wt := &mockWorktreeManager{existsResult: true, pathResult: "/tmp/existing-ws"}
	restore := setupRunTest(wt, &mockRunner{})
	defer restore()

	runBranch = "feature/exists"
	runRemoveAfter = true
	// No reader input at all — --remove-after must never read from it.
	promptReader = strings.NewReader("")

	if err := callRunE([]string{"echo", "hello"}); err != nil {
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
	wt := &mockWorktreeManager{existsResult: true, pathResult: "/tmp/existing-ws"}
	restore := setupRunTest(wt, &mockRunner{})
	defer restore()

	runBranch = "feature/exists"
	runRemoveAfter = true

	if err := callRunE([]string{"echo", "hello"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !wt.lockCalled {
		t.Error("expected Lock() to be called when --remove-after opts a reused cutting into the temporary lifecycle")
	}
	if !wt.unlockCalled {
		t.Error("expected Unlock() to be called after cleanup")
	}
}

func TestRunCmd_ExistingBranch_WithoutRemoveAfter_NoLockRegistered(t *testing.T) {
	wt := &mockWorktreeManager{existsResult: true, pathResult: "/tmp/existing-ws"}
	restore := setupRunTest(wt, &mockRunner{})
	defer restore()

	runBranch = "feature/exists"
	promptReader = strings.NewReader("n\n")

	if err := callRunE([]string{"echo", "hello"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wt.lockCalled {
		t.Error("Lock() should not be called for a reused cutting without --remove-after")
	}
	if wt.unlockCalled {
		t.Error("Unlock() should not be called for a reused cutting without --remove-after")
	}
}

// Note: the "interrupted by a real OS signal" case for a reused cutting
// (no prompt, no removal) is covered at the e2e level in e2e/run_test.go —
// run.go's sigCh is only ever fed by signal.Notify, the same reason the
// existing signal-handling tests above test cancellation semantics via
// signalAwareRun directly rather than delivering a real signal here.

func TestRunCmd_AddFails(t *testing.T) {
	wt := &mockWorktreeManager{addErr: errors.New("git error")}
	restore := setupRunTest(wt, &mockRunner{})
	defer restore()

	runBranch = "feature/new"

	err := callRunE([]string{"echo", "hello"})
	if err == nil {
		t.Fatal("expected error when Add() fails, got nil")
	}
	if !strings.Contains(err.Error(), "create cutting") {
		t.Errorf("error %q missing 'create cutting' prefix", err.Error())
	}
}

func TestRunCmd_Success_CleanupCalled(t *testing.T) {
	wt := &mockWorktreeManager{addPath: "/tmp/ws"}
	runner := &mockRunner{}
	restore := setupRunTest(wt, runner)
	defer restore()

	runBranch = "feature/foo"

	if err := callRunE([]string{"echo", "hello"}); err != nil {
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
	wt := &mockWorktreeManager{addPath: "/tmp/ws"}
	runner := &mockRunner{runErr: errors.New("command failed")}
	restore := setupRunTest(wt, runner)
	defer restore()

	runBranch = "feature/foo"

	err := callRunE([]string{"failing-cmd"})
	if err == nil {
		t.Fatal("expected error from failing command, got nil")
	}
	if !wt.removeCalled {
		t.Error("Remove() was not called after command error")
	}
}

func TestRunCmd_ExitError_CleanupCalledAndExitFnInvoked(t *testing.T) {
	wt := &mockWorktreeManager{addPath: "/tmp/ws"}

	var exitErr *exec.ExitError
	if err := exec.Command("sh", "-c", "exit 3").Run(); !errors.As(err, &exitErr) {
		t.Skip("could not construct *exec.ExitError for test")
	}

	runner := &mockRunner{runErr: exitErr}
	restore := setupRunTest(wt, runner)
	defer restore()

	runBranch = "feature/foo"

	var capturedCode int
	exitFn = func(code int) { capturedCode = code }

	err := callRunE([]string{"sh", "-c", "exit 3"})
	if err != nil {
		t.Errorf("RunE should return nil for ExitError (exit handled via exitFn), got: %v", err)
	}
	if !wt.removeCalled {
		t.Error("Remove() was not called before exitFn")
	}
	if capturedCode != 3 {
		t.Errorf("exitFn called with code %d, want 3", capturedCode)
	}
}

func TestRunCmd_ExplicitBranch(t *testing.T) {
	wt := &mockWorktreeManager{addPath: "/tmp/ws"}
	restore := setupRunTest(wt, &mockRunner{})
	defer restore()

	runBranch = "feature/my-branch"

	if err := callRunE([]string{"true"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if wt.addBranch != "feature/my-branch" {
		t.Errorf("branch = %q, want %q", wt.addBranch, "feature/my-branch")
	}
}

func TestRunCmd_FromFlagPassedToAdd(t *testing.T) {
	wt := &mockWorktreeManager{addPath: "/tmp/ws"}
	restore := setupRunTest(wt, &mockRunner{})
	defer restore()

	runBranch = "feature/new"
	runSource = "main"

	if err := callRunE([]string{"true"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if wt.addBase != "main" {
		t.Errorf("Add base = %q, want %q", wt.addBase, "main")
	}
}

func TestRunCmd_BranchExists_NoCreate(t *testing.T) {
	wt := &mockWorktreeManager{addPath: "/tmp/ws", branchExists: true}
	restore := setupRunTest(wt, &mockRunner{})
	defer restore()

	runBranch = "existing-branch"

	if err := callRunE([]string{"true"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if wt.addCreateBranch {
		t.Error("Add() called with createBranch=true for an existing branch")
	}
}

func TestRunCmd_BranchNotExists_Create(t *testing.T) {
	wt := &mockWorktreeManager{addPath: "/tmp/ws", branchExists: false}
	restore := setupRunTest(wt, &mockRunner{})
	defer restore()

	runBranch = "new-branch"

	if err := callRunE([]string{"true"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !wt.addCreateBranch {
		t.Error("Add() called with createBranch=false for a non-existing branch")
	}
}

func TestRunCmd_RunnerReceivesCorrectArgs(t *testing.T) {
	wt := &mockWorktreeManager{addPath: "/tmp/testws"}
	runner := &mockRunner{}
	restore := setupRunTest(wt, runner)
	defer restore()

	runBranch = "my-branch"

	if err := callRunE([]string{"go", "test", "./..."}); err != nil {
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
	wt := &mockWorktreeManager{addPath: "/tmp/ws"}
	restore := setupRunTest(wt, &mockRunner{})
	defer restore()

	runBranch = "feature/new"
	deps.cfg = &config.Config{DefaultBranch: "develop", RunCleanupOnSignal: true}

	if err := callRunE([]string{"true"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if wt.addBase != "develop" {
		t.Errorf("Add base = %q, want %q (config DefaultBranch)", wt.addBase, "develop")
	}
}

// --- orphan sweep ---

func TestRunCmd_SweepOrphans_CalledBeforeCreate(t *testing.T) {
	wt := &mockWorktreeManager{addDetachedPath: "/tmp/ws"}
	restore := setupRunTest(wt, &mockRunner{})
	defer restore()

	if err := callRunE([]string{"true"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(wt.callOrder) == 0 || wt.callOrder[0] != "SweepOrphans" {
		t.Errorf("call order = %v, want SweepOrphans first", wt.callOrder)
	}
}

func TestRunCmd_SweepOrphans_PrintsCleanedKeys(t *testing.T) {
	wt := &mockWorktreeManager{addDetachedPath: "/tmp/ws", sweepResult: []string{"cut-run-123"}}
	restore := setupRunTest(wt, &mockRunner{})
	defer restore()

	stdout := captureStdout(t, func() {
		if err := callRunE([]string{"true"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, "Cleaned up orphaned cutting from a previous run: cut-run-123") {
		t.Errorf("stdout = %q, want it to mention the cleaned-up orphan", stdout)
	}
}

func TestRunCmd_SweepOrphans_ErrorIsNonFatal(t *testing.T) {
	wt := &mockWorktreeManager{addDetachedPath: "/tmp/ws", sweepErr: errors.New("sweep failed")}
	restore := setupRunTest(wt, &mockRunner{})
	defer restore()

	if err := callRunE([]string{"true"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wt.addDetachedName == "" {
		t.Error("run should still proceed to create a worktree despite a sweep error")
	}
}

// --- run lock / unlock ---

func TestRunCmd_LockCalledAfterWorktreeCreated_UnlockCalledOnCleanup(t *testing.T) {
	wt := &mockWorktreeManager{addDetachedPath: "/tmp/ws"}
	restore := setupRunTest(wt, &mockRunner{})
	defer restore()

	if err := callRunE([]string{"true"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !wt.lockCalled {
		t.Error("Lock() was not called")
	}
	if !strings.HasPrefix(wt.lockKey, "cut-run-") {
		t.Errorf("Lock key = %q, want prefix %q", wt.lockKey, "cut-run-")
	}
	if !wt.unlockCalled {
		t.Error("Unlock() was not called")
	}
	if wt.unlockKey != wt.lockKey {
		t.Errorf("Unlock key = %q, want it to match Lock key %q", wt.unlockKey, wt.lockKey)
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
	wt := &mockWorktreeManager{addDetachedPath: "/tmp/ws", lockErr: errors.New("lock failed")}
	runner := &mockRunner{}
	restore := setupRunTest(wt, runner)
	defer restore()

	if err := callRunE([]string{"true"}); err != nil {
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
	wt := &mockWorktreeManager{addDetachedPath: "/tmp/ws", unlockErr: errors.New("unlock failed")}
	restore := setupRunTest(wt, &mockRunner{})
	defer restore()

	if err := callRunE([]string{"true"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !wt.removeCalled {
		t.Error("Remove() was not called")
	}
}

// --- run_cleanup_on_signal = false ---

func TestRunCmd_CleanupOnSignalDisabled_SkipsSweepLockAndUnlock(t *testing.T) {
	wt := &mockWorktreeManager{addDetachedPath: "/tmp/ws"}
	restore := setupRunTest(wt, &mockRunner{})
	defer restore()

	deps.cfg.RunCleanupOnSignal = false

	if err := callRunE([]string{"true"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if wt.sweepCalled {
		t.Error("SweepOrphans() was called despite run_cleanup_on_signal=false")
	}
	if wt.lockCalled {
		t.Error("Lock() was called despite run_cleanup_on_signal=false")
	}
	if wt.unlockCalled {
		t.Error("Unlock() was called despite run_cleanup_on_signal=false")
	}
	// Remove() is unconditional — the plain defer-based cleanup this feature
	// was layered on top of must still run regardless of the config toggle.
	if !wt.removeCalled {
		t.Error("Remove() was not called")
	}
}

func TestRunCmd_CleanupOnSignalDisabled_SignalDoesNotCancelRunningCommand(t *testing.T) {
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
	restore := setupRunTest(wt, runner)
	defer restore()

	deps.cfg.RunCleanupOnSignal = false

	done := make(chan error, 1)
	go func() { done <- callRunE([]string{"true"}) }()

	<-started
	// With the feature disabled, nothing is listening on sigCh, so this must
	// have no effect on the running command — simulate that directly by
	// confirming the command only finishes when we close(finished), not on
	// some external cancellation.
	select {
	case <-done:
		t.Fatal("callRunE returned before the command finished — signal handling should be disabled")
	case <-time.After(50 * time.Millisecond):
	}
	close(finished)

	if err := <-done; err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- signal-aware run helper (unit tests, no real OS signals involved) ---

func TestSignalAwareRun_NoSignal_ReturnsUnderlyingError(t *testing.T) {
	sigCh := make(chan os.Signal, 1)
	wantErr := errors.New("boom")

	sig, err := signalAwareRun(context.Background(), sigCh, func(_ context.Context) error {
		return wantErr
	})

	if !errors.Is(err, wantErr) {
		t.Errorf("err = %v, want %v", err, wantErr)
	}
	if sig != nil {
		t.Errorf("receivedSig = %v, want nil", sig)
	}
}

func TestSignalAwareRun_SignalDuringRun_CancelsContextAndReportsSignal(t *testing.T) {
	sigCh := make(chan os.Signal, 1)
	started := make(chan struct{})

	fn := func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}

	go func() {
		<-started
		sigCh <- syscall.SIGTERM
	}()

	sig, err := signalAwareRun(context.Background(), sigCh, fn)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if sig != syscall.SIGTERM {
		t.Errorf("receivedSig = %v, want SIGTERM", sig)
	}
}

func TestSignalExitCode(t *testing.T) {
	cases := []struct {
		sig  os.Signal
		want int
	}{
		{syscall.SIGINT, 130},
		{syscall.SIGTERM, 143},
		{syscall.SIGHUP, 129},
	}
	for _, c := range cases {
		if got := signalExitCode(c.sig); got != c.want {
			t.Errorf("signalExitCode(%v) = %d, want %d", c.sig, got, c.want)
		}
	}
}

// captureStdout runs fn with os.Stdout redirected to a pipe and returns
// everything written to it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}
