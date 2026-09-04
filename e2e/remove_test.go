//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemove_Clean(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newCutting(t, h, "feature/foo")
	wantPath := filepath.Join(dir, ".worktrees", "feature", "foo")

	r := h.run("remove", "feature/foo")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "removed")

	if _, err := os.Stat(wantPath); !os.IsNotExist(err) {
		t.Fatalf("expected worktree dir removed, stat err = %v", err)
	}
	if !branchExists(t, dir, "feature/foo") {
		t.Fatalf("branch feature/foo should be preserved after remove")
	}
}

func TestRemove_UncommittedChanges(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newCutting(t, h, "feature/foo")
	wsPath := filepath.Join(dir, ".worktrees", "feature", "foo")

	if err := os.WriteFile(filepath.Join(wsPath, "README.md"), []byte("dirty\n"), 0o600); err != nil {
		t.Fatalf("write dirty file: %v", err)
	}

	r := h.run("remove", "feature/foo")
	requireExitCode(t, r, 1)
	requireContains(t, r.stderr, "modified or untracked files")

	if _, err := os.Stat(wsPath); err != nil {
		t.Fatalf("expected worktree dir to still exist after failed remove: %v", err)
	}
}

func TestRemove_Force(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newCutting(t, h, "feature/foo")
	wsPath := filepath.Join(dir, ".worktrees", "feature", "foo")

	if err := os.WriteFile(filepath.Join(wsPath, "README.md"), []byte("dirty\n"), 0o600); err != nil {
		t.Fatalf("write dirty file: %v", err)
	}

	r := h.run("remove", "--force", "feature/foo")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "removed")

	if _, err := os.Stat(wsPath); !os.IsNotExist(err) {
		t.Fatalf("expected worktree dir removed, stat err = %v", err)
	}
}

// TestRemove_ShorthandForce verifies -f is equivalent to --force.
func TestRemove_ShorthandForce(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newCutting(t, h, "feature/foo")
	wsPath := filepath.Join(dir, ".worktrees", "feature", "foo")

	if err := os.WriteFile(filepath.Join(wsPath, "README.md"), []byte("dirty\n"), 0o600); err != nil {
		t.Fatalf("write dirty file: %v", err)
	}

	r := h.run("remove", "-f", "feature/foo")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "removed")

	if _, err := os.Stat(wsPath); !os.IsNotExist(err) {
		t.Fatalf("expected worktree dir removed, stat err = %v", err)
	}
}

func TestRemove_NotFound(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)

	r := h.run("remove", "nope")
	requireExitCode(t, r, 1)
	requireContains(t, r.stderr, "no cutting found")
}

func TestRemove_AliasParity(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newCutting(t, h, "feature/foo")

	r := h.run("rm", "feature/foo")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "removed")
}

func TestRemove_DryRun(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newCutting(t, h, "feature/foo")
	wsPath := filepath.Join(dir, ".worktrees", "feature", "foo")

	r := h.run("remove", "--dry-run", "feature/foo")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "feature/foo")

	if _, err := os.Stat(wsPath); err != nil {
		t.Fatalf("expected worktree dir to still exist after dry-run: %v", err)
	}
	if !branchExists(t, dir, "feature/foo") {
		t.Fatalf("branch feature/foo should be preserved after dry-run")
	}
}

func TestRemove_DryRun_NotFound(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)

	r := h.run("remove", "--dry-run", "nope")
	requireExitCode(t, r, 1)
	requireContains(t, r.stderr, "no cutting found")
}

func TestRemove_All(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newCutting(t, h, "feature/foo")
	newCutting(t, h, "feature/bar")
	fooPath := filepath.Join(dir, ".worktrees", "feature", "foo")
	barPath := filepath.Join(dir, ".worktrees", "feature", "bar")

	r := h.run("remove", "--all")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "feature/foo")
	requireContains(t, r.stdout, "feature/bar")

	if _, err := os.Stat(fooPath); !os.IsNotExist(err) {
		t.Fatalf("expected feature/foo worktree dir removed, stat err = %v", err)
	}
	if _, err := os.Stat(barPath); !os.IsNotExist(err) {
		t.Fatalf("expected feature/bar worktree dir removed, stat err = %v", err)
	}
	if !branchExists(t, dir, "feature/foo") {
		t.Fatalf("branch feature/foo should be preserved after remove --all")
	}
	if !branchExists(t, dir, "feature/bar") {
		t.Fatalf("branch feature/bar should be preserved after remove --all")
	}
	if !containsPath(worktreePaths(t, dir), dir) {
		t.Fatalf("expected main worktree %s to still be registered after remove --all", dir)
	}
}

func TestRemove_AllDryRun(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newCutting(t, h, "feature/foo")
	newCutting(t, h, "feature/bar")
	fooPath := filepath.Join(dir, ".worktrees", "feature", "foo")
	barPath := filepath.Join(dir, ".worktrees", "feature", "bar")

	r := h.run("remove", "--all", "--dry-run")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "feature/foo")
	requireContains(t, r.stdout, "feature/bar")

	if _, err := os.Stat(fooPath); err != nil {
		t.Fatalf("expected feature/foo worktree dir to still exist after dry-run: %v", err)
	}
	if _, err := os.Stat(barPath); err != nil {
		t.Fatalf("expected feature/bar worktree dir to still exist after dry-run: %v", err)
	}
}

func TestRemove_AllUncommittedChanges(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newCutting(t, h, "feature/foo")
	newCutting(t, h, "feature/bar")
	fooPath := filepath.Join(dir, ".worktrees", "feature", "foo")
	barPath := filepath.Join(dir, ".worktrees", "feature", "bar")

	if err := os.WriteFile(filepath.Join(barPath, "README.md"), []byte("dirty\n"), 0o600); err != nil {
		t.Fatalf("write dirty file: %v", err)
	}

	r := h.run("remove", "--all")
	requireExitCode(t, r, 1)
	requireContains(t, r.stderr, "modified or untracked files")
	requireContains(t, r.stdout, "feature/foo")

	if _, err := os.Stat(fooPath); !os.IsNotExist(err) {
		t.Fatalf("expected clean feature/foo worktree dir removed, stat err = %v", err)
	}
	if _, err := os.Stat(barPath); err != nil {
		t.Fatalf("expected dirty feature/bar worktree dir to still exist: %v", err)
	}

	rForce := h.run("remove", "--all", "--force")
	requireExitCode(t, rForce, 0)
	requireContains(t, rForce.stdout, "feature/bar")

	if _, err := os.Stat(barPath); !os.IsNotExist(err) {
		t.Fatalf("expected feature/bar worktree dir removed after --force, stat err = %v", err)
	}
}

// TestRemove_AllShorthand verifies -a is equivalent to --all.
func TestRemove_AllShorthand(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newCutting(t, h, "feature/foo")
	wsPath := filepath.Join(dir, ".worktrees", "feature", "foo")

	r := h.run("remove", "-a")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "feature/foo")

	if _, err := os.Stat(wsPath); !os.IsNotExist(err) {
		t.Fatalf("expected worktree dir removed, stat err = %v", err)
	}
}

func TestRemove_AllNoCuttings(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)

	r := h.run("remove", "--all")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "No cuttings to remove.")
}

func TestRemove_AllRejectsBranchArg(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newCutting(t, h, "feature/foo")

	r := h.run("remove", "--all", "feature/foo")
	requireExitCode(t, r, 1)
}
