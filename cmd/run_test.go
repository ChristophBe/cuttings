/*
Copyright © 2026 Christoph Becker
*/
package cmd

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/ChristophBe/workstreams/internal/config"
	"github.com/ChristophBe/workstreams/internal/worktree"
)

// --- mock implementations ---

type mockWorktreeManager struct {
	existsResult     bool
	branchExists     bool
	addPath          string
	addErr           error
	addDetachedPath  string
	addDetachedErr   error
	currentBranch    string
	currentBranchErr error
	removeErr        error

	// recorded call arguments
	addBranch       string
	addCreateBranch bool
	addBase         string
	addDetachedName string
	addDetachedBase string
	removeCalled    bool
	removeKey       string
}

func (m *mockWorktreeManager) Exists(_ string) bool       { return m.existsResult }
func (m *mockWorktreeManager) BranchExists(_ string) bool { return m.branchExists }
func (m *mockWorktreeManager) Add(branch string, createBranch bool, base string) (string, error) {
	m.addBranch = branch
	m.addCreateBranch = createBranch
	m.addBase = base
	return m.addPath, m.addErr
}
func (m *mockWorktreeManager) AddDetached(name, base string) (string, error) {
	m.addDetachedName = name
	m.addDetachedBase = base
	return m.addDetachedPath, m.addDetachedErr
}
func (m *mockWorktreeManager) CurrentBranch() (string, error) {
	return m.currentBranch, m.currentBranchErr
}
func (m *mockWorktreeManager) Remove(key string) error {
	m.removeCalled = true
	m.removeKey = key
	return m.removeErr
}
func (m *mockWorktreeManager) ListBranches() ([]string, error)    { return nil, nil }
func (m *mockWorktreeManager) List() ([]worktree.Worktree, error) { return nil, nil }
func (m *mockWorktreeManager) Path(_ string) string               { return "" }

type mockRunner struct {
	runErr error

	// recorded call arguments
	runDir     string
	runBranch  string
	runCommand []string
}

func (m *mockRunner) Run(dir, branch string, command []string) error {
	m.runDir = dir
	m.runBranch = branch
	m.runCommand = command
	return m.runErr
}

// setupRunTest replaces global deps and flags with test doubles and returns a
// restore function that must be deferred by the caller.
func setupRunTest(wt *mockWorktreeManager, runner *mockRunner) func() {
	savedDeps := deps
	savedRunBranch := runBranch
	savedRunFrom := runFrom
	savedExitFn := exitFn

	deps.cfg = &config.Config{}
	deps.wt = wt
	deps.runner = runner

	runBranch = ""
	runFrom = ""

	return func() {
		deps = savedDeps
		runBranch = savedRunBranch
		runFrom = savedRunFrom
		exitFn = savedExitFn
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
	if !strings.HasPrefix(wt.addDetachedName, "ws-run-") {
		t.Errorf("detached worktree name = %q, want prefix %q", wt.addDetachedName, "ws-run-")
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
		t.Errorf("WORKSTREAM_BRANCH = %q, want %q", runner.runBranch, "feature/foo")
	}
}

func TestRunCmd_NoBranch_FromFlagPassedToAddDetached(t *testing.T) {
	wt := &mockWorktreeManager{currentBranch: "main", addDetachedPath: "/tmp/ws"}
	restore := setupRunTest(wt, &mockRunner{})
	defer restore()

	runFrom = "origin/main"

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
	if !strings.Contains(err.Error(), "create workstream") {
		t.Errorf("error %q missing 'create workstream' prefix", err.Error())
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
	// The worktree key should be the generated ws-run-* name, not the branch name.
	if !strings.HasPrefix(wt.removeKey, "ws-run-") {
		t.Errorf("Remove key = %q, want prefix %q", wt.removeKey, "ws-run-")
	}
}

// --- explicit --branch path ---

func TestRunCmd_WorktreeAlreadyExists(t *testing.T) {
	wt := &mockWorktreeManager{existsResult: true}
	restore := setupRunTest(wt, &mockRunner{})
	defer restore()

	runBranch = "feature/exists"

	err := callRunE([]string{"echo", "hello"})
	if err == nil {
		t.Fatal("expected error when worktree already exists, got nil")
	}
	if wt.addBranch != "" {
		t.Error("Add() should not have been called")
	}
}

func TestRunCmd_AddFails(t *testing.T) {
	wt := &mockWorktreeManager{addErr: errors.New("git error")}
	restore := setupRunTest(wt, &mockRunner{})
	defer restore()

	runBranch = "feature/new"

	err := callRunE([]string{"echo", "hello"})
	if err == nil {
		t.Fatal("expected error when Add() fails, got nil")
	}
	if !strings.Contains(err.Error(), "create workstream") {
		t.Errorf("error %q missing 'create workstream' prefix", err.Error())
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
	runFrom = "main"

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
	deps.cfg = &config.Config{DefaultBranch: "develop"}

	if err := callRunE([]string{"true"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if wt.addBase != "develop" {
		t.Errorf("Add base = %q, want %q (config DefaultBranch)", wt.addBase, "develop")
	}
}
