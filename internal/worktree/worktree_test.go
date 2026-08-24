/*
Copyright © 2026 Christoph Becker
*/
package worktree_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	m := worktree.NewManager(dir, ".worktrees")

	path, err := m.Add("to-remove", true, "")
	if err != nil {
		t.Fatalf("Add() setup: %v", err)
	}

	if err := m.Remove("to-remove"); err != nil {
		t.Fatalf("Remove() unexpected error: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("worktree directory still exists after Remove()")
	}
}

func TestRemove_NotFound(t *testing.T) {
	dir := initRepo(t)
	m := worktree.NewManager(dir, ".worktrees")

	err := m.Remove("nonexistent")
	if err == nil {
		t.Fatal("Remove() expected error for nonexistent branch, got nil")
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

	path, err := m.AddDetached("ws-run-test", "")
	if err != nil {
		t.Fatalf("AddDetached() unexpected error: %v", err)
	}

	want := filepath.Join(dir, ".worktrees", "ws-run-test")
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
	path, err := m.AddDetached("ws-run-at-first", strings.TrimSpace(string(firstHead)))
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

	if _, err := m.AddDetached("ws-run-nobranche", ""); err != nil {
		t.Fatalf("AddDetached() unexpected error: %v", err)
	}

	// The generated name must NOT appear as a git branch.
	//nolint:gosec // test helper — dir is a controlled temp path
	out, err := exec.Command("git", "-C", dir, "branch", "--list", "ws-run-nobranche").Output()
	if err != nil {
		t.Fatalf("git branch --list: %v", err)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("branch %q was created but should not have been", "ws-run-nobranche")
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

// runLocksDir mirrors the package-private layout (<git-common-dir>/workstreams/run-locks)
// so tests can inspect or seed lock files without exporting internals. For a
// freshly initRepo'd, non-linked repository the common dir is simply <dir>/.git.
func runLocksDir(dir string) string {
	return filepath.Join(dir, ".git", "workstreams", "run-locks")
}

// writeRawLock writes a lock file directly (bypassing Lock, which always
// records the current process's own PID) so tests can simulate a lock left
// behind by a different, possibly-dead process.
func writeRawLock(t *testing.T, dir, name string, lock map[string]any) {
	t.Helper()
	lockDir := runLocksDir(dir)
	if err := os.MkdirAll(lockDir, 0o750); err != nil {
		t.Fatalf("MkdirAll(%q): %v", lockDir, err)
	}
	data, err := json.Marshal(lock)
	if err != nil {
		t.Fatalf("marshal lock: %v", err)
	}
	if err := os.WriteFile(filepath.Join(lockDir, name), data, 0o600); err != nil {
		t.Fatalf("write lock file: %v", err)
	}
}

func TestLock_WritesLockFile(t *testing.T) {
	dir := initRepo(t)
	m := worktree.NewManager(dir, ".worktrees")

	if err := m.Lock("some-key"); err != nil {
		t.Fatalf("Lock() unexpected error: %v", err)
	}

	entries, err := os.ReadDir(runLocksDir(dir))
	if err != nil {
		t.Fatalf("ReadDir(run-locks): %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("run-locks dir has %d entries, want 1", len(entries))
	}

	data, err := os.ReadFile(filepath.Join(runLocksDir(dir), entries[0].Name())) //nolint:gosec // test-controlled path
	if err != nil {
		t.Fatalf("read lock file: %v", err)
	}
	var lock worktree.RunLock
	if err := json.Unmarshal(data, &lock); err != nil {
		t.Fatalf("unmarshal lock file: %v", err)
	}
	if lock.Key != "some-key" {
		t.Errorf("lock.Key = %q, want %q", lock.Key, "some-key")
	}
	if lock.PID != os.Getpid() {
		t.Errorf("lock.PID = %d, want %d (current process)", lock.PID, os.Getpid())
	}
	if lock.Path != m.Path("some-key") {
		t.Errorf("lock.Path = %q, want %q", lock.Path, m.Path("some-key"))
	}
}

func TestUnlock_RemovesLockFile(t *testing.T) {
	dir := initRepo(t)
	m := worktree.NewManager(dir, ".worktrees")

	if err := m.Lock("some-key"); err != nil {
		t.Fatalf("Lock() setup: %v", err)
	}
	if err := m.Unlock("some-key"); err != nil {
		t.Fatalf("Unlock() unexpected error: %v", err)
	}

	entries, err := os.ReadDir(runLocksDir(dir))
	if err != nil {
		t.Fatalf("ReadDir(run-locks): %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("run-locks dir has %d entries after Unlock(), want 0", len(entries))
	}
}

func TestUnlock_NonexistentKey_NotAnError(t *testing.T) {
	dir := initRepo(t)
	m := worktree.NewManager(dir, ".worktrees")

	if err := m.Unlock("never-locked"); err != nil {
		t.Errorf("Unlock() unexpected error for a key that was never locked: %v", err)
	}
}

func TestSweepOrphans_NoLocks_ReturnsEmpty(t *testing.T) {
	dir := initRepo(t)
	m := worktree.NewManager(dir, ".worktrees")

	cleaned, err := m.SweepOrphans()
	if err != nil {
		t.Fatalf("SweepOrphans() unexpected error: %v", err)
	}
	if len(cleaned) != 0 {
		t.Errorf("SweepOrphans() = %v, want empty", cleaned)
	}
}

func TestSweepOrphans_LivePID_LeavesWorktreeAlone(t *testing.T) {
	dir := initRepo(t)
	m := worktree.NewManager(dir, ".worktrees")

	path, err := m.Add("still-running", true, "")
	if err != nil {
		t.Fatalf("Add() setup: %v", err)
	}
	if err := m.Lock("still-running"); err != nil {
		t.Fatalf("Lock() setup: %v", err)
	}

	cleaned, err := m.SweepOrphans()
	if err != nil {
		t.Fatalf("SweepOrphans() unexpected error: %v", err)
	}
	if len(cleaned) != 0 {
		t.Errorf("SweepOrphans() = %v, want empty (owning PID is this test process, still alive)", cleaned)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("worktree directory should still exist: %v", err)
	}
}

func TestSweepOrphans_DeadPID_RemovesWorktreeAndLock(t *testing.T) {
	dir := initRepo(t)
	m := worktree.NewManager(dir, ".worktrees")

	path, err := m.Add("to-sweep", true, "")
	if err != nil {
		t.Fatalf("Add() setup: %v", err)
	}

	// A process that has already exited — its PID is guaranteed dead.
	deadCmd := exec.Command("true")
	if err := deadCmd.Run(); err != nil {
		t.Fatalf("run sentinel process: %v", err)
	}
	deadPID := deadCmd.Process.Pid

	writeRawLock(t, dir, "dead.json", map[string]any{
		"key":       "to-sweep",
		"path":      path,
		"pid":       deadPID,
		"createdAt": time.Now(),
	})

	cleaned, err := m.SweepOrphans()
	if err != nil {
		t.Fatalf("SweepOrphans() unexpected error: %v", err)
	}
	if len(cleaned) != 1 || cleaned[0] != "to-sweep" {
		t.Errorf("SweepOrphans() = %v, want [to-sweep]", cleaned)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("worktree directory still exists after SweepOrphans()")
	}
	entries, err := os.ReadDir(runLocksDir(dir))
	if err != nil {
		t.Fatalf("ReadDir(run-locks): %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("run-locks dir has %d entries after sweep, want 0", len(entries))
	}
}

func TestSweepOrphans_CorruptLockFile_RemovedAndSkipped(t *testing.T) {
	dir := initRepo(t)
	m := worktree.NewManager(dir, ".worktrees")

	lockDir := runLocksDir(dir)
	if err := os.MkdirAll(lockDir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(lockDir, "garbage.json"), []byte("not json"), 0o600); err != nil {
		t.Fatalf("write garbage lock file: %v", err)
	}

	cleaned, err := m.SweepOrphans()
	if err != nil {
		t.Fatalf("SweepOrphans() unexpected error: %v", err)
	}
	if len(cleaned) != 0 {
		t.Errorf("SweepOrphans() = %v, want empty (corrupt file is not a valid orphan)", cleaned)
	}
	if _, err := os.Stat(filepath.Join(lockDir, "garbage.json")); !os.IsNotExist(err) {
		t.Error("corrupt lock file was not removed")
	}
}
