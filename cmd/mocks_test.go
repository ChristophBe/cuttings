/*
Copyright © 2026 Christoph Becker
*/
package cmd

import (
	"context"

	"github.com/ChristophBe/cuttings/internal/worktree"
)

// mockWorktreeManager is the package-wide test double for WorktreeManager.
// Every field is optional: a test sets only the results the command under test
// actually consumes, and inspects only the calls it cares about.
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
	// removeErrs fails only for specific branches, for bulk operations that
	// must keep going after one cutting fails. It takes precedence over
	// removeErr.
	removeErrs   map[string]error
	listResult   []worktree.Worktree
	listErr      error
	branches     []string
	branchesErr  error
	mergedResult []string
	mergedErr    error

	// recorded call arguments
	addBranch       string
	addCreateBranch bool
	addBase         string
	addDetachedName string
	addDetachedBase string
	removeCalled    bool
	removeKey       string
	removedBranches []string
	removeForces    []bool
	mergedBase      string
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

func (m *mockWorktreeManager) Remove(key string, force bool) error {
	m.callOrder = append(m.callOrder, "Remove")
	m.removeCalled = true
	m.removeKey = key
	m.removedBranches = append(m.removedBranches, key)
	m.removeForces = append(m.removeForces, force)
	if err, ok := m.removeErrs[key]; ok {
		return err
	}
	return m.removeErr
}

func (m *mockWorktreeManager) ListBranches() ([]string, error) {
	return m.branches, m.branchesErr
}

func (m *mockWorktreeManager) ListMergedBranches(base string) ([]string, error) {
	m.mergedBase = base
	return m.mergedResult, m.mergedErr
}

func (m *mockWorktreeManager) List() ([]worktree.Worktree, error) {
	return m.listResult, m.listErr
}

func (m *mockWorktreeManager) Path(_ string) string { return m.pathResult }

// mockRunner is a test double for CommandRunner.
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

// mockSpawner is a test double for ShellSpawner. Spawn never replaces the
// process here, so unlike the real spawner it simply records its arguments.
type mockSpawner struct {
	spawnErr error

	spawnCalled bool
	spawnDir    string
	spawnBranch string
}

func (m *mockSpawner) Spawn(dir, branch string) error {
	m.spawnCalled = true
	m.spawnDir = dir
	m.spawnBranch = branch
	return m.spawnErr
}

// mockLocks is a test double for the run-lock store.
type mockLocks struct {
	acquireErr  error
	releaseErr  error
	sweepResult []string
	sweepErr    error

	// recorded call arguments
	acquireCalled bool
	acquireKey    string
	acquirePath   string
	releaseCalled bool
	releaseKey    string
	sweepCalled   bool
	// sweepRemove is the removal callback Sweep was handed, so a test can
	// drive it and observe what the caller does with an orphan.
	sweepRemove func(key string) error
	// callOrder records lock operations so tests can assert ordering against
	// the worktree mock's own record.
	callOrder *[]string
}

func (m *mockLocks) record(op string) {
	if m.callOrder != nil {
		*m.callOrder = append(*m.callOrder, op)
	}
}

func (m *mockLocks) Acquire(key, path string) error {
	m.record("Lock")
	m.acquireCalled = true
	m.acquireKey = key
	m.acquirePath = path
	return m.acquireErr
}

func (m *mockLocks) Release(key string) error {
	m.record("Unlock")
	m.releaseCalled = true
	m.releaseKey = key
	return m.releaseErr
}

func (m *mockLocks) Sweep(remove func(key string) error) ([]string, error) {
	m.record("SweepOrphans")
	m.sweepCalled = true
	m.sweepRemove = remove
	return m.sweepResult, m.sweepErr
}
