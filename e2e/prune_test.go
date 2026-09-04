//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrune_MergedCutting(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newCutting(t, h, "feature/foo")
	wantPath := filepath.Join(dir, ".worktrees", "feature", "foo")

	r := h.run("prune")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "removed")

	if _, err := os.Stat(wantPath); !os.IsNotExist(err) {
		t.Fatalf("expected worktree dir removed, stat err = %v", err)
	}
	if !branchExists(t, dir, "feature/foo") {
		t.Fatalf("branch feature/foo should be preserved after prune")
	}
}

func TestPrune_NotMerged(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newCutting(t, h, "feature/foo")
	wsPath := filepath.Join(dir, ".worktrees", "feature", "foo")
	commitFile(t, wsPath, "extra.txt", "extra\n", "diverge from main")

	r := h.run("prune")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "No cuttings to prune.")

	if _, err := os.Stat(wsPath); err != nil {
		t.Fatalf("expected unmerged worktree dir to still exist: %v", err)
	}
}

func TestPrune_DryRun(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newCutting(t, h, "feature/foo")
	wsPath := filepath.Join(dir, ".worktrees", "feature", "foo")

	r := h.run("prune", "--dry-run")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "feature/foo")

	if _, err := os.Stat(wsPath); err != nil {
		t.Fatalf("expected worktree dir to still exist after dry-run: %v", err)
	}
	if !branchExists(t, dir, "feature/foo") {
		t.Fatalf("branch feature/foo should be preserved after dry-run")
	}
}

func TestPrune_UncommittedChanges(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newCutting(t, h, "feature/foo")
	wsPath := filepath.Join(dir, ".worktrees", "feature", "foo")

	if err := os.WriteFile(filepath.Join(wsPath, "README.md"), []byte("dirty\n"), 0o600); err != nil {
		t.Fatalf("write dirty file: %v", err)
	}

	r := h.run("prune")
	requireExitCode(t, r, 1)
	requireContains(t, r.stderr, "modified or untracked files")

	if _, err := os.Stat(wsPath); err != nil {
		t.Fatalf("expected worktree dir to still exist after failed prune: %v", err)
	}
}

func TestPrune_Force(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newCutting(t, h, "feature/foo")
	wsPath := filepath.Join(dir, ".worktrees", "feature", "foo")

	if err := os.WriteFile(filepath.Join(wsPath, "README.md"), []byte("dirty\n"), 0o600); err != nil {
		t.Fatalf("write dirty file: %v", err)
	}

	r := h.run("prune", "--force")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "removed")

	if _, err := os.Stat(wsPath); !os.IsNotExist(err) {
		t.Fatalf("expected worktree dir removed, stat err = %v", err)
	}
}

func TestPrune_DefaultBranchConfig(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)

	// Give "develop" a commit main doesn't have, so a cutting forked from it
	// is merged into develop but not into main.
	runGit(t, dir, "checkout", "-q", "-b", "develop")
	commitFile(t, dir, "develop.txt", "develop\n", "develop-only commit")
	runGit(t, dir, "checkout", "-q", "main")

	newCutting(t, h, "feature/on-develop", "--source", "develop")
	writeConfig(t, dir, "default_branch: develop\n")

	r := h.run("prune")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "removed")

	wantPath := filepath.Join(dir, ".worktrees", "feature", "on-develop")
	if _, err := os.Stat(wantPath); !os.IsNotExist(err) {
		t.Fatalf("expected worktree dir removed, stat err = %v", err)
	}
}

func TestPrune_PreservesMain(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newCutting(t, h, "feature/foo")
	wsPath := filepath.Join(dir, ".worktrees", "feature", "bar")
	newCutting(t, h, "feature/bar")
	commitFile(t, wsPath, "extra.txt", "extra\n", "diverge from main")

	r := h.run("prune")
	requireExitCode(t, r, 0)

	if !containsPath(worktreePaths(t, dir), dir) {
		t.Fatalf("expected main worktree %s to still be registered after prune", dir)
	}
}

func TestPrune_NoCuttings(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)

	r := h.run("prune")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "No cuttings to prune.")
}
