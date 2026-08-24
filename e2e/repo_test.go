//go:build e2e

package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitEnvVars are cleared before running fixture git commands so tests behave
// correctly even when this suite itself runs from within a git hook (which
// sets GIT_DIR, GIT_INDEX_FILE, etc.) — mirrors
// internal/worktree/worktree_test.go's initRepo.
var gitEnvVars = []string{
	"GIT_DIR",
	"GIT_INDEX_FILE",
	"GIT_WORK_TREE",
	"GIT_OBJECT_DIRECTORY",
	"GIT_COMMON_DIR",
}

// runGit runs git with args in dir, failing the test on error. It is used
// for fixture setup and for verifying repository state directly — never for
// invoking the workstreams binary itself (use harness.run for that), so
// assertions stay genuinely black-box.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	for _, v := range gitEnvVars {
		t.Setenv(v, "")
		_ = os.Unsetenv(v)
	}
	//nolint:gosec // args are test-controlled literals, not external input.
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out.String())
	}
	return out.String()
}

// realPath resolves symlinks in p. On macOS /var is a symlink to
// /private/var, so t.TempDir() and paths reported by git can otherwise look
// different while referring to the same directory.
func realPath(t *testing.T, p string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", p, err)
	}
	return resolved
}

// initRepo creates a temporary git repository with a "main" branch and one
// commit, and returns its (symlink-resolved) root path.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := realPath(t, t.TempDir())

	runGit(t, dir, "init", "-q", "-b", "main")
	runGit(t, dir, "config", "user.email", "e2e@example.com")
	runGit(t, dir, "config", "user.name", "Workstreams E2E")

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# fixture\n"), 0o600); err != nil {
		t.Fatalf("write README.md: %v", err)
	}
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-q", "-m", "initial commit")

	return dir
}

// commitFile writes name with content in dir and commits it.
func commitFile(t *testing.T, dir, name, content, message string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	runGit(t, dir, "add", name)
	runGit(t, dir, "commit", "-q", "-m", message)
}

// branchExists reports whether branch exists locally in the repo at dir.
func branchExists(t *testing.T, dir, branch string) bool {
	t.Helper()
	//nolint:gosec // branch is test-controlled, not external input.
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	cmd.Dir = dir
	return cmd.Run() == nil
}

// worktreePaths returns the "worktree <path>" entries reported by
// `git worktree list --porcelain` for the repo at dir, one per registered
// worktree (including the main one).
func worktreePaths(t *testing.T, dir string) []string {
	t.Helper()
	out := runGit(t, dir, "worktree", "list", "--porcelain")
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		if p, ok := strings.CutPrefix(line, "worktree "); ok {
			paths = append(paths, p)
		}
	}
	return paths
}

// containsPath reports whether path is present in paths.
func containsPath(paths []string, path string) bool {
	for _, p := range paths {
		if p == path {
			return true
		}
	}
	return false
}

// fakeShellPath returns the absolute path to the non-interactive $SHELL
// fixture used by tests that exercise `new`/`shell`.
func fakeShellPath() string {
	return filepath.Join(repoRoot, "e2e", "testdata", "fakeshell.sh")
}

// readConfigFile returns the content of .workstreams.yaml at dir.
func readConfigFile(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, ".workstreams.yaml")
	//nolint:gosec // path is built from a test fixture directory, not external input.
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}
