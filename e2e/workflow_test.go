//go:build e2e

package e2e

import (
	"path/filepath"
	"testing"
)

// TestWorkflow_InitNewListRemove exercises the flagship end-to-end lifecycle:
// init -> new -> list -> remove -> list.
func TestWorkflow_InitNewListRemove(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)

	requireExitCode(t, h.run("init"), 0)

	newWorkstream(t, h, "feature/foo")

	afterNew := h.run("list")
	requireExitCode(t, afterNew, 0)
	requireContains(t, afterNew.stdout, "feature/foo")

	remove := h.run("remove", "feature/foo")
	requireExitCode(t, remove, 0)

	afterRemove := h.run("list")
	requireExitCode(t, afterRemove, 0)
	requireNotContains(t, afterRemove.stdout, "feature/foo")

	if !branchExists(t, dir, "feature/foo") {
		t.Fatalf("branch feature/foo should still exist after remove")
	}

	wantPath := filepath.Join(dir, ".worktrees", "feature", "foo")
	if containsPath(worktreePaths(t, dir), wantPath) {
		t.Fatalf("worktree %s should have been unregistered after remove", wantPath)
	}
}
