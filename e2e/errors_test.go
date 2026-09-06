//go:build e2e

package e2e

import (
	"strings"
	"testing"
)

// TestErrorOutput_ErrorPrecedesUsage pins the shape of a failed command's
// output: Cobra reports the error first and the usage after it. This is easy to
// invert accidentally when the error reporting moves between Cobra and
// cmd.Execute, and nothing else in the suite would notice.
func TestErrorOutput_ErrorPrecedesUsage(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)

	r := h.run("shell", "no-such-cutting")
	requireExitCode(t, r, 1)

	combined := r.stdout + r.stderr
	errIdx := strings.Index(combined, `Error: no cutting found for branch "no-such-cutting"`)
	usageIdx := strings.Index(combined, "Usage:")

	if errIdx < 0 {
		t.Fatalf("no error line in output:\nstdout:\n%s\nstderr:\n%s", r.stdout, r.stderr)
	}
	if usageIdx < 0 {
		t.Fatalf("no usage in output:\nstdout:\n%s\nstderr:\n%s", r.stdout, r.stderr)
	}
	if errIdx > usageIdx {
		t.Errorf("the error line appears after the usage block; want it first:\n%s", combined)
	}
}

// TestErrorOutput_ArgumentValidation covers the same shape for an error raised
// by the Args validator rather than by RunE.
func TestErrorOutput_ArgumentValidation(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)

	r := h.run("run")
	requireExitCode(t, r, 1)

	combined := r.stdout + r.stderr
	requireContains(t, combined, `Error: requires a command to run after "--"`)
	requireContains(t, combined, "Usage:")
}

// TestRun_NonZeroExit_ReportsNothingExtra verifies that a command exiting
// non-zero inside a cutting looks exactly like running it directly: its exit
// code is propagated, and cuttings adds neither an "Error:" line of its own nor
// a usage block. Only the cleanup notice belongs there.
func TestRun_NonZeroExit_ReportsNothingExtra(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)

	r := h.run("run", "--", "sh", "-c", "exit 42")

	requireExitCode(t, r, 42)
	requireContains(t, r.stdout, "Cleaning up cutting")
	requireNotContains(t, r.stdout+r.stderr, "Error:")
	requireNotContains(t, r.stdout+r.stderr, "Usage:")
	requireNotContains(t, r.stdout+r.stderr, "exit status")
}

// A cuttings-level failure (as opposed to the command failing) still exits 1
// and reports the reason.
func TestRun_CommandNotFound_IsACuttingsError(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)

	r := h.run("run", "--", "definitely-not-a-real-binary")

	requireExitCode(t, r, 1)
	requireContains(t, r.stdout+r.stderr, "Error:")
	// The temporary cutting is still cleared away.
	requireContains(t, r.stdout, "Cleaning up cutting")
	after := worktreePaths(t, dir)
	if len(after) != 1 {
		t.Errorf("worktrees after a failed command = %v, want only the main worktree", after)
	}
}
