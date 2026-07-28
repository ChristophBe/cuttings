/*
Copyright © 2026 Christoph Becker
*/
package worktree_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ChristophBe/workstreams/internal/worktree"
)

// realPath resolves symlinks in a path. On macOS /var is a symlink to
// /private/var, so t.TempDir() and git rev-parse may return different-looking
// but equivalent paths.
func realPath(t *testing.T, p string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", p, err)
	}
	return resolved
}

// initRepo creates a temporary git repository with an initial commit and
// returns its root path. The repo is suitable for worktree operations.
// gitEnvVars lists git environment variables that are set when running inside
// a git hook and must be cleared so that test git repos are isolated.
var gitEnvVars = []string{
	"GIT_DIR",
	"GIT_INDEX_FILE",
	"GIT_WORK_TREE",
	"GIT_OBJECT_DIRECTORY",
	"GIT_COMMON_DIR",
}

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Clear git hook environment variables so tests run correctly from within
	// a pre-commit hook (which sets GIT_DIR, GIT_INDEX_FILE, etc.).
	for _, v := range gitEnvVars {
		t.Setenv(v, "")
		if err := os.Unsetenv(v); err != nil {
			t.Fatalf("unsetenv %s: %v", v, err)
		}
	}

	run := func(args ...string) {
		t.Helper()
		//nolint:gosec // test helper — args are controlled literals, not user input.
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run("init", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")

	// An initial commit is required before worktrees can be created.
	readmePath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", "README.md")
	run("commit", "-m", "initial commit")

	return dir
}

func TestFindRepoRoot(t *testing.T) {
	dir := initRepo(t)

	// Change into the repo so FindRepoRoot can find it.
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	root, err := worktree.FindRepoRoot()
	if err != nil {
		t.Fatalf("FindRepoRoot() unexpected error: %v", err)
	}
	// Resolve symlinks on both sides — on macOS /var is a symlink to /private/var.
	if realPath(t, root) != realPath(t, dir) {
		t.Errorf("FindRepoRoot() = %q, want %q", root, dir)
	}
}

func TestAdd_NewBranch(t *testing.T) {
	dir := initRepo(t)

	path, err := worktree.Add(dir, "feature/test", true, "")
	if err != nil {
		t.Fatalf("Add() unexpected error: %v", err)
	}

	want := filepath.Join(dir, ".worktrees", "feature", "test")
	if path != want {
		t.Errorf("Add() path = %q, want %q", path, want)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("worktree directory does not exist: %v", err)
	}
}

func TestAdd_ExistingBranch(t *testing.T) {
	dir := initRepo(t)

	// Create the branch first.
	cmd := exec.Command("git", "branch", "existing-branch")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create branch: %v\n%s", err, out)
	}

	path, err := worktree.Add(dir, "existing-branch", false, "")
	if err != nil {
		t.Fatalf("Add() unexpected error: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("worktree directory does not exist: %v", err)
	}
}

func TestList(t *testing.T) {
	dir := initRepo(t)

	trees, err := worktree.List(dir)
	if err != nil {
		t.Fatalf("List() unexpected error: %v", err)
	}

	if len(trees) == 0 {
		t.Fatal("List() returned no worktrees; expected at least the main worktree")
	}

	main := trees[0]
	if !main.IsMain {
		t.Errorf("first worktree IsMain = false, want true")
	}
	if main.Branch != "main" {
		t.Errorf("main worktree Branch = %q, want %q", main.Branch, "main")
	}
}

func TestList_WithWorktree(t *testing.T) {
	dir := initRepo(t)

	if _, err := worktree.Add(dir, "feature/listed", true, ""); err != nil {
		t.Fatalf("Add() setup: %v", err)
	}

	trees, err := worktree.List(dir)
	if err != nil {
		t.Fatalf("List() unexpected error: %v", err)
	}

	if len(trees) < 2 {
		t.Fatalf("List() returned %d worktrees, want at least 2", len(trees))
	}

	found := false
	for _, tr := range trees {
		if tr.Branch == "feature/listed" {
			found = true
			if tr.IsMain {
				t.Errorf("workstream worktree has IsMain = true")
			}
		}
	}
	if !found {
		t.Error("List() did not include the added workstream")
	}
}

func TestRemove(t *testing.T) {
	dir := initRepo(t)

	path, err := worktree.Add(dir, "to-remove", true, "")
	if err != nil {
		t.Fatalf("Add() setup: %v", err)
	}

	if err := worktree.Remove(dir, "to-remove"); err != nil {
		t.Fatalf("Remove() unexpected error: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("worktree directory still exists after Remove()")
	}
}

func TestRemove_NotFound(t *testing.T) {
	dir := initRepo(t)

	err := worktree.Remove(dir, "nonexistent")
	if err == nil {
		t.Fatal("Remove() expected error for nonexistent branch, got nil")
	}
}

func TestExists(t *testing.T) {
	dir := initRepo(t)

	if worktree.Exists(dir, "feature/check") {
		t.Error("Exists() = true before Add(), want false")
	}

	if _, err := worktree.Add(dir, "feature/check", true, ""); err != nil {
		t.Fatalf("Add() setup: %v", err)
	}

	if !worktree.Exists(dir, "feature/check") {
		t.Error("Exists() = false after Add(), want true")
	}
}
