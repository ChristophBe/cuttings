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
	newWorkstream(t, h, "feature/foo")
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
	newWorkstream(t, h, "feature/foo")
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
	newWorkstream(t, h, "feature/foo")
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
	newWorkstream(t, h, "feature/foo")
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
	requireContains(t, r.stderr, "no workstream found")
}

func TestRemove_AliasParity(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newWorkstream(t, h, "feature/foo")

	r := h.run("rm", "feature/foo")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "removed")
}
