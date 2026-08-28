//go:build e2e

package e2e

import (
	"path/filepath"
	"testing"
)

func TestRun_DetachedCleansUpOnSuccess(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)

	before := worktreePaths(t, dir)

	r := h.run("run", "--", "sh", "-c", "echo hello-from-run; pwd")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "hello-from-run")
	requireContains(t, r.stdout, filepath.Join(dir, ".worktrees", "ws-run-"))

	after := worktreePaths(t, dir)
	if len(after) != len(before) {
		t.Fatalf("worktree not cleaned up: before=%v after=%v", before, after)
	}
}

func TestRun_ExitCodePropagationAndCleanup(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)

	before := worktreePaths(t, dir)

	r := h.run("run", "--", "sh", "-c", "exit 7")
	requireExitCode(t, r, 7)
	requireContains(t, r.stdout, "Cleaning up workstream")

	after := worktreePaths(t, dir)
	if len(after) != len(before) {
		t.Fatalf("worktree not cleaned up after failing command: before=%v after=%v", before, after)
	}
}

func TestRun_Branch(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)

	r := h.run("run", "--branch", "feature/run", "--", "sh", "-c", "echo ran")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "ran")

	if !branchExists(t, dir, "feature/run") {
		t.Fatalf("expected branch feature/run to be created")
	}
	wantPath := filepath.Join(dir, ".worktrees", "feature", "run")
	if containsPath(worktreePaths(t, dir), wantPath) {
		t.Fatalf("expected worktree %s to be cleaned up, branch should persist without it", wantPath)
	}
}

// TestRun_ExistingBranch_RunsInPlace verifies that `run --branch <existing>`
// reuses the existing worktree in place (no new worktree is created) and
// wires up the environment the same as for a freshly-created one.
func TestRun_ExistingBranch_RunsInPlace(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newWorkstream(t, h, "feature/foo")

	before := worktreePaths(t, dir)

	r := h.run("run", "--branch", "feature/foo", "--", "sh", "-c", "echo BRANCH=$WORKSTREAM_BRANCH")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "Using existing workstream")
	requireContains(t, r.stdout, "BRANCH=feature/foo")

	after := worktreePaths(t, dir)
	if len(after) != len(before) {
		t.Fatalf("expected no new worktree to be created: before=%v after=%v", before, after)
	}
}

// TestRun_ExistingBranch_DefaultKeepsOnEOF verifies that with no terminal
// attached (stdin hits immediate EOF, as with the plain run() helper), the
// removal prompt defaults to "no" and the reused workstream survives.
func TestRun_ExistingBranch_DefaultKeepsOnEOF(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newWorkstream(t, h, "feature/foo")

	r := h.run("run", "--branch", "feature/foo", "--", "true")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, `Leaving workstream "feature/foo" in place`)

	wantPath := filepath.Join(dir, ".worktrees", "feature", "foo")
	if !containsPath(worktreePaths(t, dir), wantPath) {
		t.Fatalf("expected existing workstream %s to survive the default no-answer", wantPath)
	}
}

// TestRun_ExistingBranch_PromptRemove_Yes verifies answering "y" to the
// removal prompt removes the reused workstream after the command finishes.
func TestRun_ExistingBranch_PromptRemove_Yes(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newWorkstream(t, h, "feature/foo")

	r := h.runWithStdin("y\n", "run", "--branch", "feature/foo", "--", "true")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, `Removing workstream "feature/foo"`)

	wantPath := filepath.Join(dir, ".worktrees", "feature", "foo")
	if containsPath(worktreePaths(t, dir), wantPath) {
		t.Fatalf("expected existing workstream %s to be removed after confirming", wantPath)
	}
	if !branchExists(t, dir, "feature/foo") {
		t.Fatalf("expected branch feature/foo to be preserved (only the worktree is removed)")
	}
}

// TestRun_ExistingBranch_PromptRemove_No mirrors the "y" case for an
// explicit "n" answer.
func TestRun_ExistingBranch_PromptRemove_No(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newWorkstream(t, h, "feature/foo")

	r := h.runWithStdin("n\n", "run", "--branch", "feature/foo", "--", "true")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, `Leaving workstream "feature/foo" in place`)

	wantPath := filepath.Join(dir, ".worktrees", "feature", "foo")
	if !containsPath(worktreePaths(t, dir), wantPath) {
		t.Fatalf("expected existing workstream %s to survive an explicit no", wantPath)
	}
}

// TestRun_ExistingBranch_RemoveAfterFlag verifies --remove-after removes the
// reused workstream without ever prompting, even with no stdin available.
func TestRun_ExistingBranch_RemoveAfterFlag(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newWorkstream(t, h, "feature/foo")

	r := h.run("run", "--branch", "feature/foo", "--remove-after", "--", "true")
	requireExitCode(t, r, 0)
	requireNotContains(t, r.stdout, "Remove workstream")

	wantPath := filepath.Join(dir, ".worktrees", "feature", "foo")
	if containsPath(worktreePaths(t, dir), wantPath) {
		t.Fatalf("expected existing workstream %s to be removed via --remove-after", wantPath)
	}
}

// TestRun_ExistingBranch_RemoveAfterShorthand verifies -r is equivalent to
// --remove-after.
func TestRun_ExistingBranch_RemoveAfterShorthand(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newWorkstream(t, h, "feature/foo")

	r := h.run("run", "--branch", "feature/foo", "-r", "--", "true")
	requireExitCode(t, r, 0)

	wantPath := filepath.Join(dir, ".worktrees", "feature", "foo")
	if containsPath(worktreePaths(t, dir), wantPath) {
		t.Fatalf("expected existing workstream %s to be removed via -r", wantPath)
	}
}

// TestRun_ExistingBranch_CommandFailure_StillPrompts verifies the removal
// prompt still runs when the command inside the reused workstream fails,
// and that the command's exit code is still propagated.
func TestRun_ExistingBranch_CommandFailure_StillPrompts(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newWorkstream(t, h, "feature/foo")

	r := h.runWithStdin("y\n", "run", "--branch", "feature/foo", "--", "sh", "-c", "exit 7")
	requireExitCode(t, r, 7)
	requireContains(t, r.stdout, `Removing workstream "feature/foo"`)

	wantPath := filepath.Join(dir, ".worktrees", "feature", "foo")
	if containsPath(worktreePaths(t, dir), wantPath) {
		t.Fatalf("expected existing workstream %s to be removed after confirming despite command failure", wantPath)
	}
}

func TestRun_Source(t *testing.T) {
	dir := initRepo(t)
	runGit(t, dir, "checkout", "-b", "alt")
	commitFile(t, dir, "alt-only.txt", "alt content\n", "add alt-only file")
	runGit(t, dir, "checkout", "main")

	h := newHarness(t, dir)
	r := h.run("run", "--source", "alt", "--", "cat", "alt-only.txt")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "alt content")
}

func TestRun_EnvVars_Detached(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)

	r := h.run("run", "--", "sh", "-c", "echo BRANCH=$WORKSTREAM_BRANCH")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "BRANCH=main")
}

func TestRun_EnvVars_Branch(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)

	r := h.run("run", "--branch", "feature/env", "--", "sh", "-c", "echo BRANCH=$WORKSTREAM_BRANCH")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "BRANCH=feature/env")
}

// TestRun_ShorthandBranchAndSource verifies -b and -s are equivalent to
// --branch and --source.
func TestRun_ShorthandBranchAndSource(t *testing.T) {
	dir := initRepo(t)
	runGit(t, dir, "checkout", "-b", "alt")
	commitFile(t, dir, "alt-only.txt", "alt content\n", "add alt-only file")
	runGit(t, dir, "checkout", "main")

	h := newHarness(t, dir)
	r := h.run("run", "-b", "feature/short", "-s", "alt", "--", "cat", "alt-only.txt")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "alt content")

	if !branchExists(t, dir, "feature/short") {
		t.Fatalf("expected branch feature/short to be created via -b")
	}
}
