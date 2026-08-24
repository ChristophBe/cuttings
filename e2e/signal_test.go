//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// TestRun_SignalCleanup_SIGINT verifies that a real SIGINT delivered to
// `workstreams run` while its command is executing still cleans up the
// temporary worktree (via the signal-aware cancellation added in
// cmd/run.go), and reports the shell exit-code convention (128+signum).
func TestRun_SignalCleanup_SIGINT(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	before := worktreePaths(t, dir)

	proc := h.start("run", "--", "sleep", "30")
	waitForWorktreeCount(t, dir, len(before)+1, 5*time.Second)
	proc.signal(syscall.SIGINT)
	r := proc.wait()

	requireExitCode(t, r, 130) // 128 + SIGINT(2)
	requireContains(t, r.stdout, "Cleaning up workstream")

	after := worktreePaths(t, dir)
	if len(after) != len(before) {
		t.Fatalf("worktree not cleaned up after SIGINT: before=%v after=%v", before, after)
	}
}

// TestRun_SignalCleanup_SIGTERM mirrors TestRun_SignalCleanup_SIGINT for
// SIGTERM, the signal a process manager typically sends first.
func TestRun_SignalCleanup_SIGTERM(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	before := worktreePaths(t, dir)

	proc := h.start("run", "--", "sleep", "30")
	waitForWorktreeCount(t, dir, len(before)+1, 5*time.Second)
	proc.signal(syscall.SIGTERM)
	r := proc.wait()

	requireExitCode(t, r, 143) // 128 + SIGTERM(15)
	requireContains(t, r.stdout, "Cleaning up workstream")

	after := worktreePaths(t, dir)
	if len(after) != len(before) {
		t.Fatalf("worktree not cleaned up after SIGTERM: before=%v after=%v", before, after)
	}
}

// TestRun_CleanupOnSignalDisabled_SignalLeavesOrphan verifies that with
// run_cleanup_on_signal=false, run installs no signal handling at all — a
// SIGINT applies Go's default (uncaught) disposition, which terminates the
// process immediately without running any cleanup defers, leaving the
// worktree orphaned. This is what distinguishes the config flag from a no-op.
func TestRun_CleanupOnSignalDisabled_SignalLeavesOrphan(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir).withEnv("WORKSTREAMS_RUN_CLEANUP_ON_SIGNAL", "false")
	before := worktreePaths(t, dir)

	// A short sleep: with no signal handling installed, SIGINT kills
	// `workstreams` immediately, but the orphaned "sleep" grandchild inherits
	// the same stdout pipe workstreams was using — proc.wait() below can't
	// see EOF (and so can't return) until that pipe's write end closes, which
	// only happens once "sleep" itself exits.
	proc := h.start("run", "--", "sleep", "2")
	after := waitForWorktreeCount(t, dir, len(before)+1, 5*time.Second)
	proc.signal(syscall.SIGINT)
	r := proc.wait()

	if r.exitCode != signalTerminatedExitCode {
		t.Fatalf("exit code = %d, want %d (signal-terminated)\nstdout:\n%s", r.exitCode, signalTerminatedExitCode, r.stdout)
	}
	requireNotContains(t, r.stdout, "Cleaning up workstream")

	stillThere := worktreePaths(t, dir)
	if len(stillThere) != len(after) {
		t.Fatalf("expected the worktree to remain orphaned when cleanup-on-signal is disabled: before-signal=%v after-signal=%v", after, stillThere)
	}

	// Tidy up the orphan directly via git so it doesn't leak state into other
	// tests (none currently share this repo dir, but keep the fixture clean).
	for _, p := range stillThere {
		if !containsPath(before, p) {
			runGit(t, dir, "worktree", "remove", "--force", p)
		}
	}
}

// TestRun_OrphanSweep_CleansUpOnNextRun seeds a run-lock file (as Lock would
// write) pointing at an existing worktree, owned by a PID that's no longer
// alive, and verifies the next `workstreams run` invocation's orphan sweep
// (SweepOrphans, called at the top of RunE when run_cleanup_on_signal is
// enabled) removes both the stale worktree and its lock file.
func TestRun_OrphanSweep_CleansUpOnNextRun(t *testing.T) {
	dir := initRepo(t)
	orphanKey := "orphan-key"
	orphanPath := filepath.Join(dir, ".worktrees", orphanKey)
	runGit(t, dir, "worktree", "add", "--detach", orphanPath)
	lockPath := writeOrphanRunLock(t, dir, orphanKey, orphanPath)

	h := newHarness(t, dir)
	r := h.run("run", "--", "true")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "Cleaned up orphaned workstream from a previous run: "+orphanKey)

	if _, err := os.Stat(orphanPath); !os.IsNotExist(err) {
		t.Fatalf("expected orphaned worktree removed, stat err = %v", err)
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("expected stale lock file removed, stat err = %v", err)
	}
}

// TestRun_CleanupOnSignalDisabledViaConfigFile_SkipsOrphanSweep verifies the
// run_cleanup_on_signal=false config *file* setting (not just the env var
// override) disables the orphan sweep, leaving a seeded orphan untouched.
func TestRun_CleanupOnSignalDisabledViaConfigFile_SkipsOrphanSweep(t *testing.T) {
	dir := initRepo(t)
	writeConfig(t, dir, "run_cleanup_on_signal: false\n")

	orphanKey := "orphan-key"
	orphanPath := filepath.Join(dir, ".worktrees", orphanKey)
	runGit(t, dir, "worktree", "add", "--detach", orphanPath)
	lockPath := writeOrphanRunLock(t, dir, orphanKey, orphanPath)

	h := newHarness(t, dir)
	r := h.run("run", "--", "true")
	requireExitCode(t, r, 0)
	requireNotContains(t, r.stdout, "Cleaned up orphaned workstream")

	if _, err := os.Stat(orphanPath); err != nil {
		t.Fatalf("expected orphaned worktree left untouched when cleanup-on-signal is disabled: %v", err)
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("expected stale lock file left untouched when cleanup-on-signal is disabled: %v", err)
	}
}
