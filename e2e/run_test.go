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

func TestRun_BranchAlreadyExists(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newWorkstream(t, h, "feature/foo")

	r := h.run("run", "--branch", "feature/foo", "--", "true")
	requireExitCode(t, r, 1)
	requireContains(t, r.stderr, "already exists")
}

func TestRun_From(t *testing.T) {
	dir := initRepo(t)
	runGit(t, dir, "checkout", "-b", "alt")
	commitFile(t, dir, "alt-only.txt", "alt content\n", "add alt-only file")
	runGit(t, dir, "checkout", "main")

	h := newHarness(t, dir)
	r := h.run("run", "--from", "alt", "--", "cat", "alt-only.txt")
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
