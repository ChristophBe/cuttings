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
// `cuttings run` while its command is executing still cleans up the
// temporary worktree (via the signal-aware cancellation in internal/run),
// and reports the shell exit-code convention (128+signum).
func TestRun_SignalCleanup_SIGINT(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	before := worktreePaths(t, dir)

	command, started := blockingCommand(t)
	proc := h.start(append([]string{"run", "--"}, command...)...)
	waitForFile(t, started, 5*time.Second)
	proc.signal(syscall.SIGINT)
	r := proc.wait()

	requireExitCode(t, r, 130) // 128 + SIGINT(2)
	requireContains(t, r.stdout, "Cleaning up cutting")

	requireWorktrees(t, dir, before)
}

// TestRun_SignalCleanup_SIGTERM mirrors TestRun_SignalCleanup_SIGINT for
// SIGTERM, the signal a process manager typically sends first.
func TestRun_SignalCleanup_SIGTERM(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	before := worktreePaths(t, dir)

	command, started := blockingCommand(t)
	proc := h.start(append([]string{"run", "--"}, command...)...)
	waitForFile(t, started, 5*time.Second)
	proc.signal(syscall.SIGTERM)
	r := proc.wait()

	requireExitCode(t, r, 143) // 128 + SIGTERM(15)
	requireContains(t, r.stdout, "Cleaning up cutting")

	requireWorktrees(t, dir, before)
}

// TestRun_CleanupOnSignalDisabled_SignalLeavesOrphan verifies that with
// run_cleanup_on_signal=false, run installs no signal handling at all — a
// SIGINT applies Go's default (uncaught) disposition, which terminates the
// process immediately without running any cleanup defers, leaving the
// worktree orphaned. This is what distinguishes the config flag from a no-op.
func TestRun_CleanupOnSignalDisabled_SignalLeavesOrphan(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir).withEnv("CUTTINGS_RUN_CLEANUP_ON_SIGNAL", "false")
	before := worktreePaths(t, dir)

	command, started := blockingCommand(t)
	proc := h.start(append([]string{"run", "--"}, command...)...)
	waitForFile(t, started, 5*time.Second)
	orphaned := worktreePaths(t, dir)
	proc.signal(syscall.SIGINT)
	r := proc.wait()

	if r.exitCode != signalTerminatedExitCode {
		t.Fatalf("exit code = %d, want %d (signal-terminated)\nstdout:\n%s", r.exitCode, signalTerminatedExitCode, r.stdout)
	}
	requireNotContains(t, r.stdout, "Cleaning up cutting")

	// The worktree that existed when the signal was sent must still be there.
	requireWorktrees(t, dir, orphaned)

	// Tidy up the orphan directly via git so it doesn't leak state into other
	// tests (none currently share this repo dir, but keep the fixture clean).
	for _, p := range orphaned {
		if !containsPath(before, p) {
			runGit(t, dir, "worktree", "remove", "--force", p)
		}
	}
}

// TestRun_OrphanSweep_CleansUpOnNextRun seeds a run-lock file (as Lock would
// write) pointing at an existing worktree, owned by a PID that's no longer
// alive, and verifies the next `cuttings run` invocation's orphan sweep
// (run.Provisioner.SweepOrphans, called before provisioning when
// run_cleanup_on_signal is enabled) removes both the stale worktree and its
// lock file.
func TestRun_OrphanSweep_CleansUpOnNextRun(t *testing.T) {
	dir := initRepo(t)
	orphanKey := "orphan-key"
	orphanPath := filepath.Join(dir, ".worktrees", orphanKey)
	runGit(t, dir, "worktree", "add", "--detach", orphanPath)
	lockPath := writeOrphanRunLock(t, dir, orphanKey, orphanPath)

	h := newHarness(t, dir)
	r := h.run("run", "--", "true")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "Cleaned up orphaned cutting from a previous run: "+orphanKey)

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
	requireNotContains(t, r.stdout, "Cleaned up orphaned cutting")

	if _, err := os.Stat(orphanPath); err != nil {
		t.Fatalf("expected orphaned worktree left untouched when cleanup-on-signal is disabled: %v", err)
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("expected stale lock file left untouched when cleanup-on-signal is disabled: %v", err)
	}
}

// TestRun_ExistingBranch_SignalLeavesInPlace verifies that a SIGINT
// delivered while `run --branch <existing>` is executing does NOT remove
// the reused cutting (unlike a freshly-created one) — since it was never
// locked or registered for the temporary-worktree cleanup path, a signal is
// treated as leaving real, non-temporary work exactly where it was.
func TestRun_ExistingBranch_SignalLeavesInPlace(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newCutting(t, h, "feature/foo")
	before := worktreePaths(t, dir)

	command, started := blockingCommand(t)
	proc := h.start(append([]string{"run", "--branch", "feature/foo", "--"}, command...)...)
	waitForFile(t, started, 5*time.Second)
	proc.signal(syscall.SIGINT)
	r := proc.wait()

	requireExitCode(t, r, 130) // 128 + SIGINT(2)
	requireNotContains(t, r.stdout, "Remove cutting")

	requireWorktrees(t, dir, before)
}

// TestRun_ExistingBranch_RemoveAfterFlag_SignalStillCleansUp verifies the
// opposite of TestRun_ExistingBranch_SignalLeavesInPlace: with --remove-after,
// a reused cutting opts into the full temporary-worktree safety net, so a
// SIGINT still cleans it up exactly like a freshly-created one would.
func TestRun_ExistingBranch_RemoveAfterFlag_SignalStillCleansUp(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newCutting(t, h, "feature/foo")
	before := worktreePaths(t, dir)
	// Resolve while it still exists — the point of the test is that it won't.
	cuttingPath := realPath(t, filepath.Join(dir, ".worktrees", "feature", "foo"))

	command, started := blockingCommand(t)
	proc := h.start(append([]string{"run", "--branch", "feature/foo", "--remove-after", "--"}, command...)...)
	waitForFile(t, started, 5*time.Second)
	proc.signal(syscall.SIGINT)
	r := proc.wait()

	requireExitCode(t, r, 130) // 128 + SIGINT(2)
	requireContains(t, r.stdout, "Cleaning up cutting")

	requireWorktrees(t, dir, without(before, cuttingPath))
	if !branchExists(t, dir, "feature/foo") {
		t.Fatalf("expected branch feature/foo to be preserved (only the worktree is removed)")
	}
}
