/*
Copyright © 2026 Christoph Becker
*/
package worktree_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ChristophBe/cuttings/internal/worktree"
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

// clearGitEnv clears the git environment variables that are set when running
// inside a git hook (as pre-commit does), so a test sees only the repository it
// created. Without it, a test can inherit GIT_DIR from the hook and silently
// operate on the real repository.
func clearGitEnv(t *testing.T) {
	t.Helper()
	for _, v := range gitEnvVars {
		t.Setenv(v, "")
		if err := os.Unsetenv(v); err != nil {
			t.Fatalf("unsetenv %s: %v", v, err)
		}
	}
}

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	clearGitEnv(t)

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
	m := worktree.NewManager(dir, ".worktrees")

	path, err := m.Add("feature/test", true, "")
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
	m := worktree.NewManager(dir, ".worktrees")

	// Create the branch first.
	cmd := exec.Command("git", "branch", "existing-branch")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create branch: %v\n%s", err, out)
	}

	path, err := m.Add("existing-branch", false, "")
	if err != nil {
		t.Fatalf("Add() unexpected error: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("worktree directory does not exist: %v", err)
	}
}

func TestList(t *testing.T) {
	dir := initRepo(t)
	m := worktree.NewManager(dir, ".worktrees")

	trees, err := m.List()
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
	m := worktree.NewManager(dir, ".worktrees")

	if _, err := m.Add("feature/listed", true, ""); err != nil {
		t.Fatalf("Add() setup: %v", err)
	}

	trees, err := m.List()
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
				t.Errorf("cutting worktree has IsMain = true")
			}
		}
	}
	if !found {
		t.Error("List() did not include the added cutting")
	}
}

func TestRemove(t *testing.T) {
	dir := initRepo(t)
	m := worktree.NewManager(dir, ".worktrees")

	path, err := m.Add("to-remove", true, "")
	if err != nil {
		t.Fatalf("Add() setup: %v", err)
	}

	if err := m.Remove("to-remove", false); err != nil {
		t.Fatalf("Remove() unexpected error: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("worktree directory still exists after Remove()")
	}
}

func TestRemove_NotFound(t *testing.T) {
	dir := initRepo(t)
	m := worktree.NewManager(dir, ".worktrees")

	err := m.Remove("nonexistent", false)
	if err == nil {
		t.Fatal("Remove() expected error for nonexistent branch, got nil")
	}
}

func TestRemove_DirtyWorktree_RequiresForce(t *testing.T) {
	dir := initRepo(t)
	m := worktree.NewManager(dir, ".worktrees")

	path, err := m.Add("to-remove", true, "")
	if err != nil {
		t.Fatalf("Add() setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(path, "README.md"), []byte("dirty\n"), 0o600); err != nil {
		t.Fatalf("write dirty file: %v", err)
	}

	if err := m.Remove("to-remove", false); err == nil {
		t.Fatal("Remove(force=false) expected error for dirty worktree, got nil")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected worktree dir to still exist after failed remove: %v", err)
	}

	if err := m.Remove("to-remove", true); err != nil {
		t.Fatalf("Remove(force=true) unexpected error: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("worktree directory still exists after Remove(force=true)")
	}
}

func TestAdd_NewBranch_WithBase(t *testing.T) {
	dir := initRepo(t)

	// Create a second commit on main so it diverges from "stable".
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

	// Record main's current HEAD as the "stable" point.
	//nolint:gosec // test helper — dir is a controlled temp path, not user input.
	stableHead, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}

	// Add a second commit to main, advancing it past the stable point.
	extraFile := filepath.Join(dir, "extra.txt")
	if err := os.WriteFile(extraFile, []byte("extra\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", "extra.txt")
	run("commit", "-m", "second commit")

	// Create a "stable" branch pointing at the first commit (before the second commit).
	run("branch", "stable", strings.TrimSpace(string(stableHead)))

	// Add a worktree from the "stable" base — the new branch should NOT include the second commit.
	m := worktree.NewManager(dir, ".worktrees")
	path, err := m.Add("feature/from-stable", true, "stable")
	if err != nil {
		t.Fatalf("Add() unexpected error: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("worktree directory does not exist: %v", err)
	}

	// The worktree's HEAD should match the stable branch tip, not main.
	//nolint:gosec // test helper — path is a controlled worktree path, not user input.
	wtHead, err := exec.Command("git", "-C", path, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse HEAD in worktree: %v", err)
	}
	if strings.TrimSpace(string(wtHead)) != strings.TrimSpace(string(stableHead)) {
		t.Errorf("worktree HEAD = %q, want stable HEAD %q", strings.TrimSpace(string(wtHead)), strings.TrimSpace(string(stableHead)))
	}
}

func TestAdd_CustomWorktreesDir(t *testing.T) {
	dir := initRepo(t)
	const customDir = ".custom-worktrees"
	m := worktree.NewManager(dir, customDir)

	path, err := m.Add("feature/custom", true, "")
	if err != nil {
		t.Fatalf("Add() unexpected error: %v", err)
	}

	want := filepath.Join(dir, customDir, "feature", "custom")
	if path != want {
		t.Errorf("Add() path = %q, want %q", path, want)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("worktree directory does not exist: %v", err)
	}
}

func TestListBranches_ReturnsCurrentBranch(t *testing.T) {
	dir := initRepo(t)
	m := worktree.NewManager(dir, ".worktrees")

	branches, err := m.ListBranches()
	if err != nil {
		t.Fatalf("ListBranches() unexpected error: %v", err)
	}
	if len(branches) != 1 || branches[0] != "main" {
		t.Errorf("ListBranches() = %v, want [main]", branches)
	}
}

func TestListBranches_ReturnsAllBranches(t *testing.T) {
	dir := initRepo(t)
	m := worktree.NewManager(dir, ".worktrees")

	run := func(args ...string) {
		t.Helper()
		//nolint:gosec // test helper — args are controlled literals, not user input.
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("branch", "feature/a")
	run("branch", "feature/b")

	branches, err := m.ListBranches()
	if err != nil {
		t.Fatalf("ListBranches() unexpected error: %v", err)
	}

	want := map[string]bool{"main": true, "feature/a": true, "feature/b": true}
	if len(branches) != len(want) {
		t.Fatalf("ListBranches() returned %d branches, want %d: %v", len(branches), len(want), branches)
	}
	for _, b := range branches {
		if !want[b] {
			t.Errorf("unexpected branch %q in ListBranches()", b)
		}
	}
}

func TestListBranches_DetachedHeadNotListed(t *testing.T) {
	dir := initRepo(t)
	m := worktree.NewManager(dir, ".worktrees")

	// Create a detached HEAD worktree; its "HEAD" should not appear in branches.
	if _, err := m.AddDetached("ws-detached", ""); err != nil {
		t.Fatalf("AddDetached() setup: %v", err)
	}

	branches, err := m.ListBranches()
	if err != nil {
		t.Fatalf("ListBranches() unexpected error: %v", err)
	}
	for _, b := range branches {
		if b == "HEAD" || b == "ws-detached" {
			t.Errorf("ListBranches() should not contain %q", b)
		}
	}
}

func TestListMergedBranches_MergedIncluded(t *testing.T) {
	dir := initRepo(t)
	m := worktree.NewManager(dir, ".worktrees")

	run := func(args ...string) {
		t.Helper()
		//nolint:gosec // test helper — args are controlled literals, not user input.
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	// feature/merged forks from main with no new commits, so it's trivially
	// merged into main.
	run("branch", "feature/merged")

	branches, err := m.ListMergedBranches("main")
	if err != nil {
		t.Fatalf("ListMergedBranches() unexpected error: %v", err)
	}

	want := map[string]bool{"main": true, "feature/merged": true}
	if len(branches) != len(want) {
		t.Fatalf("ListMergedBranches() returned %d branches, want %d: %v", len(branches), len(want), branches)
	}
	for _, b := range branches {
		if !want[b] {
			t.Errorf("unexpected branch %q in ListMergedBranches()", b)
		}
	}
}

func TestListMergedBranches_UnmergedExcluded(t *testing.T) {
	dir := initRepo(t)
	m := worktree.NewManager(dir, ".worktrees")

	if _, err := m.Add("feature/unmerged", true, ""); err != nil {
		t.Fatalf("Add() setup: %v", err)
	}
	// Commit inside the new worktree so the branch diverges from main.
	wtPath := m.Path("feature/unmerged")
	if err := os.WriteFile(filepath.Join(wtPath, "extra.txt"), []byte("extra\n"), 0o600); err != nil {
		t.Fatalf("write extra.txt: %v", err)
	}
	run := func(args ...string) {
		t.Helper()
		//nolint:gosec // test helper — args are controlled literals, not user input.
		cmd := exec.Command("git", args...)
		cmd.Dir = wtPath
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("add", "extra.txt")
	run("commit", "-m", "diverge from main")

	branches, err := m.ListMergedBranches("main")
	if err != nil {
		t.Fatalf("ListMergedBranches() unexpected error: %v", err)
	}
	for _, b := range branches {
		if b == "feature/unmerged" {
			t.Errorf("ListMergedBranches() should not contain unmerged branch %q", b)
		}
	}
}

func TestListMergedBranches_CustomBase(t *testing.T) {
	dir := initRepo(t)
	m := worktree.NewManager(dir, ".worktrees")

	run := func(runDir string, args ...string) {
		t.Helper()
		//nolint:gosec // test helper — args are controlled literals, not user input.
		cmd := exec.Command("git", args...)
		cmd.Dir = runDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	// Give "develop" a commit main doesn't have, so branches forked from it
	// are merged into develop but not into main.
	if _, err := m.Add("develop", true, ""); err != nil {
		t.Fatalf("Add() setup: %v", err)
	}
	developPath := m.Path("develop")
	if err := os.WriteFile(filepath.Join(developPath, "develop-only.txt"), []byte("develop\n"), 0o600); err != nil {
		t.Fatalf("write develop-only.txt: %v", err)
	}
	run(developPath, "add", "develop-only.txt")
	run(developPath, "commit", "-m", "develop-only commit")

	if _, err := m.Add("feature/on-develop", true, "develop"); err != nil {
		t.Fatalf("Add() setup: %v", err)
	}

	branches, err := m.ListMergedBranches("develop")
	if err != nil {
		t.Fatalf("ListMergedBranches() unexpected error: %v", err)
	}
	found := false
	for _, b := range branches {
		if b == "feature/on-develop" {
			found = true
		}
	}
	if !found {
		t.Errorf("ListMergedBranches(%q) = %v, want to contain %q", "develop", branches, "feature/on-develop")
	}

	// feature/on-develop must not show up as merged into main, since main
	// doesn't have the "develop" branch commit.
	mainBranches, err := m.ListMergedBranches("main")
	if err != nil {
		t.Fatalf("ListMergedBranches() unexpected error: %v", err)
	}
	for _, b := range mainBranches {
		if b == "feature/on-develop" || b == "develop" {
			t.Errorf("ListMergedBranches(%q) should not contain %q", "main", b)
		}
	}
}

func TestCurrentBranch(t *testing.T) {
	dir := initRepo(t)
	m := worktree.NewManager(dir, ".worktrees")

	branch, err := m.CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch() unexpected error: %v", err)
	}
	if branch != "main" {
		t.Errorf("CurrentBranch() = %q, want %q", branch, "main")
	}
}

func TestAddDetached(t *testing.T) {
	dir := initRepo(t)
	m := worktree.NewManager(dir, ".worktrees")

	path, err := m.AddDetached("cut-run-test", "")
	if err != nil {
		t.Fatalf("AddDetached() unexpected error: %v", err)
	}

	want := filepath.Join(dir, ".worktrees", "cut-run-test")
	if path != want {
		t.Errorf("AddDetached() path = %q, want %q", path, want)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("worktree directory does not exist: %v", err)
	}
}

func TestAddDetached_WithBase(t *testing.T) {
	dir := initRepo(t)

	// Add a second commit so HEAD differs from the first.
	run := func(args ...string) {
		t.Helper()
		//nolint:gosec // test helper — args are controlled literals, not user input.
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	firstHead, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output() //nolint:gosec // test helper
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}
	extraFile := filepath.Join(dir, "extra.txt")
	if err := os.WriteFile(extraFile, []byte("extra\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", "extra.txt")
	run("commit", "-m", "second commit")

	m := worktree.NewManager(dir, ".worktrees")
	path, err := m.AddDetached("cut-run-at-first", strings.TrimSpace(string(firstHead)))
	if err != nil {
		t.Fatalf("AddDetached() unexpected error: %v", err)
	}

	// The worktree HEAD should be the first commit, not the second.
	//nolint:gosec // test helper — path is a controlled worktree path
	wtHead, err := exec.Command("git", "-C", path, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse HEAD in worktree: %v", err)
	}
	if strings.TrimSpace(string(wtHead)) != strings.TrimSpace(string(firstHead)) {
		t.Errorf("worktree HEAD = %q, want %q", strings.TrimSpace(string(wtHead)), strings.TrimSpace(string(firstHead)))
	}
}

func TestAddDetached_NoBranchCreated(t *testing.T) {
	dir := initRepo(t)
	m := worktree.NewManager(dir, ".worktrees")

	if _, err := m.AddDetached("cut-run-nobranche", ""); err != nil {
		t.Fatalf("AddDetached() unexpected error: %v", err)
	}

	// The generated name must NOT appear as a git branch.
	//nolint:gosec // test helper — dir is a controlled temp path
	out, err := exec.Command("git", "-C", dir, "branch", "--list", "cut-run-nobranche").Output()
	if err != nil {
		t.Fatalf("git branch --list: %v", err)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("branch %q was created but should not have been", "cut-run-nobranche")
	}
}

func TestExists(t *testing.T) {
	dir := initRepo(t)
	m := worktree.NewManager(dir, ".worktrees")

	if m.Exists("feature/check") {
		t.Error("Exists() = true before Add(), want false")
	}

	if _, err := m.Add("feature/check", true, ""); err != nil {
		t.Fatalf("Add() setup: %v", err)
	}

	if !m.Exists("feature/check") {
		t.Error("Exists() = false after Add(), want true")
	}
}

// --- run locks / orphan sweep ---

// runLocksDir mirrors the package-private layout (<git-common-dir>/cuttings/run-locks)
// so tests can inspect or seed lock files without exporting internals. For a
// freshly initRepo'd, non-linked repository the common dir is simply <dir>/.git.
func TestCuttings(t *testing.T) {
	t.Parallel()

	trees := []worktree.Worktree{
		{Branch: "main", Path: "/repo", IsMain: true},
		{Branch: "feature/a", Path: "/repo/.worktrees/feature/a"},
		{Branch: "", Path: "/repo/.worktrees/cut-run-123"}, // detached "run" worktree
		{Branch: "feature/b", Path: "/repo/.worktrees/feature/b"},
	}

	got := worktree.Cuttings(trees)
	if len(got) != 2 {
		t.Fatalf("worktree.Cuttings() returned %d entries (%v), want 2", len(got), got)
	}
	if got[0].Branch != "feature/a" || got[1].Branch != "feature/b" {
		t.Errorf("worktree.Cuttings() = %v, want feature/a and feature/b in order", got)
	}
}

func TestCuttings_NoneLeft(t *testing.T) {
	t.Parallel()

	if got := worktree.Cuttings([]worktree.Worktree{{Branch: "main", IsMain: true}}); got != nil {
		t.Errorf("worktree.Cuttings() = %v, want nil", got)
	}
}

// GitCommonDir must resolve to the same shared .git directory from a linked
// worktree as from the main one — that is what keeps per-repo state (run
// locks) in a single place no matter where a command is invoked.
func TestGitCommonDir_SharedAcrossWorktrees(t *testing.T) {
	dir := initRepo(t)
	m := worktree.NewManager(dir, ".worktrees")

	path, err := m.Add("feature/foo", true, "")
	if err != nil {
		t.Fatalf("Add() setup: %v", err)
	}

	fromMain, err := m.GitCommonDir()
	if err != nil {
		t.Fatalf("GitCommonDir() from main worktree: %v", err)
	}
	fromCutting, err := worktree.NewManager(path, ".worktrees").GitCommonDir()
	if err != nil {
		t.Fatalf("GitCommonDir() from cutting: %v", err)
	}

	if realPath(t, fromMain) != realPath(t, fromCutting) {
		t.Errorf("GitCommonDir() = %q from main but %q from a cutting, want the same directory", fromMain, fromCutting)
	}
	if realPath(t, fromMain) != realPath(t, filepath.Join(dir, ".git")) {
		t.Errorf("GitCommonDir() = %q, want the repository's .git directory", fromMain)
	}
}

func TestGitCommonDir_NotARepo(t *testing.T) {
	// Must clear the hook environment explicitly: this test deliberately has no
	// repository of its own, so an inherited GIT_DIR would let git resolve a
	// common directory anyway and the test would pass for the wrong reason.
	clearGitEnv(t)

	m := worktree.NewManager(t.TempDir(), ".worktrees")
	if _, err := m.GitCommonDir(); err == nil {
		t.Error("GitCommonDir() error = nil outside a git repository, want an error")
	}
}
