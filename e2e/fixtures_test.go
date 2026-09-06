//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// newCutting runs `cuttings new <branch> [args...]` with the fake
// shell fixture and fails the test unless it succeeds. Used by tests that
// need an existing cutting as setup rather than exercising `new` itself.
func newCutting(t *testing.T, h *harness, branch string, extraArgs ...string) result {
	t.Helper()
	args := append([]string{"new", branch}, extraArgs...)
	r := h.withEnv("SHELL", fakeShellPath()).run(args...)
	requireExitCode(t, r, 0)
	return r
}

// blockingBackstopSeconds bounds how long a blocking command can run if its
// keepalive file somehow outlives the test. It only matters when something has
// already gone wrong; normally the command exits as soon as the test ends.
const blockingBackstopSeconds = 30

// blockingPollHz is how often the blocking command checks its keepalive file.
const blockingPollHz = 20 // one check every 50ms

// blockingCommand returns the argv of a command that announces itself by
// creating a marker file and then keeps running until the test ends, along with
// the path of that marker.
//
// Tests that signal a running `cuttings run` need the command to still be
// running when the signal lands. Waiting on a fixed `sleep` makes that a race
// against the clock — and waiting for the new worktree to appear is worse,
// because `git worktree add` registers a worktree before it has finished
// creating it, so the signal can land mid-add. The marker is written by the
// command itself, which proves both that the add completed and that the command
// is running; wait for it with waitForFile.
//
// The command stops when a keepalive file goes away, and that file lives in the
// test's own t.TempDir() — so the framework removing that directory at the end
// of the test IS the stop signal. Watching for a file to *disappear* rather than
// appear is what makes this reliable: the obvious inverse, where cleanup creates
// a release file, races the directory removal that follows it and loses, leaving
// the command polling for a path that can never exist again.
func blockingCommand(t *testing.T) (args []string, started string) {
	t.Helper()

	dir := t.TempDir()
	started = filepath.Join(dir, "started")
	keepalive := filepath.Join(dir, "keepalive")

	if err := os.WriteFile(keepalive, nil, 0o600); err != nil {
		t.Fatalf("write keepalive file: %v", err)
	}

	script := fmt.Sprintf(
		"touch %s; i=0; while [ $i -lt %d ]; do [ -f %s ] || exit 0; i=$((i+1)); sleep 0.05; done",
		started, blockingBackstopSeconds*blockingPollHz, keepalive,
	)
	return []string{"sh", "-c", script}, started
}
