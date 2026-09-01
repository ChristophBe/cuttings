//go:build e2e

package e2e

import (
	"os"
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
	requireContains(t, r.stdout, filepath.Join(dir, ".worktrees", "cut-run-"))

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
	requireContains(t, r.stdout, "Cleaning up cutting")

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
	newCutting(t, h, "feature/foo")

	before := worktreePaths(t, dir)

	r := h.run("run", "--branch", "feature/foo", "--", "sh", "-c", "echo BRANCH=$CUTTING_BRANCH")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "Using existing cutting")
	requireContains(t, r.stdout, "BRANCH=feature/foo")

	after := worktreePaths(t, dir)
	if len(after) != len(before) {
		t.Fatalf("expected no new worktree to be created: before=%v after=%v", before, after)
	}
}

// TestRun_ExistingBranch_DefaultKeepsOnEOF verifies that with no terminal
// attached (stdin hits immediate EOF, as with the plain run() helper), the
// removal prompt defaults to "no" and the reused cutting survives.
func TestRun_ExistingBranch_DefaultKeepsOnEOF(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newCutting(t, h, "feature/foo")

	r := h.run("run", "--branch", "feature/foo", "--", "true")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, `Leaving cutting "feature/foo" in place`)

	wantPath := filepath.Join(dir, ".worktrees", "feature", "foo")
	if !containsPath(worktreePaths(t, dir), wantPath) {
		t.Fatalf("expected existing cutting %s to survive the default no-answer", wantPath)
	}
}

// TestRun_ExistingBranch_PromptRemove_Yes verifies answering "y" to the
// removal prompt removes the reused cutting after the command finishes.
func TestRun_ExistingBranch_PromptRemove_Yes(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newCutting(t, h, "feature/foo")

	r := h.runWithStdin("y\n", "run", "--branch", "feature/foo", "--", "true")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, `Removing cutting "feature/foo"`)

	wantPath := filepath.Join(dir, ".worktrees", "feature", "foo")
	if containsPath(worktreePaths(t, dir), wantPath) {
		t.Fatalf("expected existing cutting %s to be removed after confirming", wantPath)
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
	newCutting(t, h, "feature/foo")

	r := h.runWithStdin("n\n", "run", "--branch", "feature/foo", "--", "true")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, `Leaving cutting "feature/foo" in place`)

	wantPath := filepath.Join(dir, ".worktrees", "feature", "foo")
	if !containsPath(worktreePaths(t, dir), wantPath) {
		t.Fatalf("expected existing cutting %s to survive an explicit no", wantPath)
	}
}

// TestRun_ExistingBranch_RemoveAfterFlag verifies --remove-after removes the
// reused cutting without ever prompting, even with no stdin available.
func TestRun_ExistingBranch_RemoveAfterFlag(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newCutting(t, h, "feature/foo")

	r := h.run("run", "--branch", "feature/foo", "--remove-after", "--", "true")
	requireExitCode(t, r, 0)
	requireNotContains(t, r.stdout, "Remove cutting")

	wantPath := filepath.Join(dir, ".worktrees", "feature", "foo")
	if containsPath(worktreePaths(t, dir), wantPath) {
		t.Fatalf("expected existing cutting %s to be removed via --remove-after", wantPath)
	}
}

// TestRun_ExistingBranch_RemoveAfterShorthand verifies -r is equivalent to
// --remove-after.
func TestRun_ExistingBranch_RemoveAfterShorthand(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newCutting(t, h, "feature/foo")

	r := h.run("run", "--branch", "feature/foo", "-r", "--", "true")
	requireExitCode(t, r, 0)

	wantPath := filepath.Join(dir, ".worktrees", "feature", "foo")
	if containsPath(worktreePaths(t, dir), wantPath) {
		t.Fatalf("expected existing cutting %s to be removed via -r", wantPath)
	}
}

// TestRun_ExistingBranch_CommandFailure_StillPrompts verifies the removal
// prompt still runs when the command inside the reused cutting fails,
// and that the command's exit code is still propagated.
func TestRun_ExistingBranch_CommandFailure_StillPrompts(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newCutting(t, h, "feature/foo")

	r := h.runWithStdin("y\n", "run", "--branch", "feature/foo", "--", "sh", "-c", "exit 7")
	requireExitCode(t, r, 7)
	requireContains(t, r.stdout, `Removing cutting "feature/foo"`)

	wantPath := filepath.Join(dir, ".worktrees", "feature", "foo")
	if containsPath(worktreePaths(t, dir), wantPath) {
		t.Fatalf("expected existing cutting %s to be removed after confirming despite command failure", wantPath)
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

	r := h.run("run", "--", "sh", "-c", "echo BRANCH=$CUTTING_BRANCH")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "BRANCH=main")
}

func TestRun_EnvVars_Branch(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)

	r := h.run("run", "--branch", "feature/env", "--", "sh", "-c", "echo BRANCH=$CUTTING_BRANCH")
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

// --- --in-place ---

func TestRun_InPlace_RunsInCurrentWorktree_NoNewWorktreeCreated(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)

	before := worktreePaths(t, dir)

	r := h.run("run", "--in-place", "--", "sh", "-c", "echo BRANCH=$CUTTING_BRANCH; pwd")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "BRANCH=main")
	requireContains(t, r.stdout, dir)

	after := worktreePaths(t, dir)
	if len(after) != len(before) {
		t.Fatalf("expected no new worktree to be created: before=%v after=%v", before, after)
	}
}

// TestRun_InPlace_Shorthand verifies -i is equivalent to --in-place.
func TestRun_InPlace_Shorthand(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)

	before := worktreePaths(t, dir)

	r := h.run("run", "-i", "--", "sh", "-c", "echo BRANCH=$CUTTING_BRANCH")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "BRANCH=main")

	after := worktreePaths(t, dir)
	if len(after) != len(before) {
		t.Fatalf("expected no new worktree to be created: before=%v after=%v", before, after)
	}
}

// TestRun_InPlace_ReusesDirtyCuttingWithoutCleaningUp verifies --in-place runs
// directly inside an existing, uncommitted-changes cutting (simulating the
// motivating use case: a dev server started via --in-place alongside a second
// shell actively editing the same files) and leaves it untouched afterward.
func TestRun_InPlace_ReusesDirtyCuttingWithoutCleaningUp(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newCutting(t, h, "feature/foo")

	cuttingPath := filepath.Join(dir, ".worktrees", "feature", "foo")
	if err := os.WriteFile(filepath.Join(cuttingPath, "dirty.txt"), []byte("uncommitted\n"), 0o600); err != nil {
		t.Fatalf("write dirty.txt: %v", err)
	}

	before := worktreePaths(t, dir)

	cuttingHarness := newHarness(t, cuttingPath)
	r := cuttingHarness.run("run", "--in-place", "--", "sh", "-c", "echo BRANCH=$CUTTING_BRANCH")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "BRANCH=feature/foo")

	after := worktreePaths(t, dir)
	if len(after) != len(before) {
		t.Fatalf("expected no worktree to be created or removed: before=%v after=%v", before, after)
	}
	if _, err := os.Stat(filepath.Join(cuttingPath, "dirty.txt")); err != nil {
		t.Fatalf("expected uncommitted change to survive --in-place: %v", err)
	}
}

func TestRun_InPlace_RejectsBranchFlag(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)

	r := h.run("run", "--in-place", "--branch", "feature/foo", "--", "true")
	requireExitCode(t, r, 1)
	requireContains(t, r.stderr, "in-place")
	requireContains(t, r.stderr, "branch")
}

func TestRun_InPlace_RejectsSourceFlag(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)

	r := h.run("run", "--in-place", "--source", "main", "--", "true")
	requireExitCode(t, r, 1)
	requireContains(t, r.stderr, "in-place")
	requireContains(t, r.stderr, "source")
}

func TestRun_InPlace_RejectsRemoveAfterFlag(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)

	r := h.run("run", "--in-place", "--remove-after", "--", "true")
	requireExitCode(t, r, 1)
	requireContains(t, r.stderr, "in-place")
	requireContains(t, r.stderr, "remove-after")
}
