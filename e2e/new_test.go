//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNew_FreshBranch(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir).withEnv("SHELL", fakeShellPath())

	r := h.run("new", "feature/foo")
	requireExitCode(t, r, 0)

	wantPath := filepath.Join(dir, ".worktrees", "feature", "foo")
	requireContains(t, r.stdout, `Creating cutting for branch "feature/foo"`)
	requireContains(t, r.stdout, "Opening shell")
	requireContains(t, r.stdout, "CUTTING_BRANCH=feature/foo")
	requireContains(t, r.stdout, "CUTTING_PATH="+wantPath)
	requireContains(t, r.stdout, "PWD="+wantPath)

	if !branchExists(t, dir, "feature/foo") {
		t.Fatalf("branch feature/foo was not created")
	}
	if !containsPath(worktreePaths(t, dir), wantPath) {
		t.Fatalf("worktree %s not registered in git", wantPath)
	}
}

func TestNew_Source(t *testing.T) {
	dir := initRepo(t)
	runGit(t, dir, "checkout", "-b", "alt")
	commitFile(t, dir, "alt-only.txt", "alt content\n", "add alt-only file")
	runGit(t, dir, "checkout", "main")

	h := newHarness(t, dir)
	newCutting(t, h, "foo", "--source", "alt")

	wantPath := filepath.Join(dir, ".worktrees", "foo")
	if _, err := os.Stat(filepath.Join(wantPath, "alt-only.txt")); err != nil {
		t.Fatalf("expected alt-only.txt in worktree forked from alt: %v", err)
	}
}

func TestNew_AlreadyExists(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newCutting(t, h, "foo")

	r := h.withEnv("SHELL", fakeShellPath()).run("new", "foo")
	requireExitCode(t, r, 1)
	requireContains(t, r.stderr, "already exists")
	requireContains(t, r.stderr, "shell foo")
}

func TestNew_BranchExistsNoCutting(t *testing.T) {
	dir := initRepo(t)
	runGit(t, dir, "branch", "existing-branch")
	shaBefore := strings.TrimSpace(runGit(t, dir, "rev-parse", "existing-branch"))

	h := newHarness(t, dir)
	newCutting(t, h, "existing-branch")

	shaAfter := strings.TrimSpace(runGit(t, dir, "rev-parse", "existing-branch"))
	if shaBefore != shaAfter {
		t.Fatalf("branch commit changed: before=%s after=%s", shaBefore, shaAfter)
	}

	wantPath := filepath.Join(dir, ".worktrees", "existing-branch")
	if !containsPath(worktreePaths(t, dir), wantPath) {
		t.Fatalf("worktree %s not registered in git", wantPath)
	}
}

func TestNew_NestedBranchName(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newCutting(t, h, "feature/foo/bar")

	wantPath := filepath.Join(dir, ".worktrees", "feature", "foo", "bar")
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("expected nested worktree dir at %s: %v", wantPath, err)
	}
}

func TestNew_OutsideRepo(t *testing.T) {
	dir := realPath(t, t.TempDir())
	h := newHarness(t, dir)

	r := h.run("new", "foo")
	requireExitCode(t, r, 1)
	requireContains(t, r.stderr, "not a git repository")
}

// TestNew_ShorthandSource verifies -s is equivalent to --source.
func TestNew_ShorthandSource(t *testing.T) {
	dir := initRepo(t)
	runGit(t, dir, "checkout", "-b", "alt")
	commitFile(t, dir, "alt-only.txt", "alt content\n", "add alt-only file")
	runGit(t, dir, "checkout", "main")

	h := newHarness(t, dir)
	newCutting(t, h, "foo", "-s", "alt")

	wantPath := filepath.Join(dir, ".worktrees", "foo")
	if _, err := os.Stat(filepath.Join(wantPath, "alt-only.txt")); err != nil {
		t.Fatalf("expected alt-only.txt in worktree forked from alt via -s: %v", err)
	}
}
