//go:build e2e

package e2e

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
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

// gitEnv returns this process's environment with the gitEnvVars removed. It
// builds a fresh slice rather than unsetting the variables globally: runGit is
// called from nearly every test, and a helper that mutates process-wide state
// is a hazard waiting to be tripped over.
func gitEnv() []string {
	var env []string
	for _, e := range os.Environ() {
		name, _, _ := strings.Cut(e, "=")
		if slices.Contains(gitEnvVars, name) {
			continue
		}
		env = append(env, e)
	}
	return env
}

// runGit runs git with args in dir, failing the test on error. It is used
// for fixture setup and for verifying repository state directly — never for
// invoking the cuttings binary itself (use harness.run for that), so
// assertions stay genuinely black-box.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	//nolint:gosec // args are test-controlled literals, not external input.
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = gitEnv()
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
	runGit(t, dir, "config", "user.name", "Cuttings E2E")

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

// requireWorktrees fails the test unless the repo at dir has exactly the given
// worktrees, naming the paths that were unexpectedly added or removed. Tests
// compare sets rather than counts so a failure says which worktree moved — the
// question anyone debugging one of these actually has.
func requireWorktrees(t *testing.T, dir string, want []string) {
	t.Helper()
	got := worktreePaths(t, dir)

	var added, missing []string
	for _, p := range got {
		if !containsPath(want, p) {
			added = append(added, p)
		}
	}
	for _, p := range want {
		if !containsPath(got, p) {
			missing = append(missing, p)
		}
	}
	if len(added) == 0 && len(missing) == 0 {
		return
	}
	t.Fatalf("unexpected worktrees:\n  unexpectedly present: %v\n  unexpectedly gone:    %v\n  (have %v, want %v)",
		added, missing, got, want)
}

// without returns paths with p removed, for building the expected worktree set
// after a cutting is removed.
func without(paths []string, p string) []string {
	var out []string
	for _, candidate := range paths {
		if candidate != p {
			out = append(out, candidate)
		}
	}
	return out
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

// readConfigFile returns the content of .cuttings.yaml at dir.
func readConfigFile(t *testing.T, dir string) string {
	t.Helper()
	return readFile(t, filepath.Join(dir, ".cuttings.yaml"))
}

// readFile returns the content of the file at path, built from a test
// fixture directory rather than external input.
func readFile(t *testing.T, path string) string {
	t.Helper()
	//nolint:gosec // path is built from a test fixture directory, not external input.
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

// waitForFile polls for path to exist, or fails the test after timeout. Used
// to synchronize with a background `cuttings run` invocation (started via
// harness.start) whose command touches a marker file once running — needed
// when no new worktree appears to poll for instead (e.g. reusing an existing
// one), and reading the process's stdout directly would race its own
// in-flight writes.
func waitForFile(t *testing.T, path string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if _, err := os.Stat(path); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s to exist", path)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// gitCommonDir returns the repo at dir's shared .git directory, resolving
// linked-worktree ".git" files the same way internal/worktree.Manager does
// for locating run-lock storage.
func gitCommonDir(t *testing.T, dir string) string {
	t.Helper()
	return strings.TrimSpace(runGit(t, dir, "rev-parse", "--path-format=absolute", "--git-common-dir"))
}

// runLockFileName derives the same filesystem-safe file name that
// internal/runlock.Store.Acquire uses for a run lock keyed by key: the first
// 8 bytes of its SHA-256 hash, hex-encoded, with a ".json" suffix.
func runLockFileName(key string) string {
	sum := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%x.json", sum[:8])
}

// writeOrphanRunLock seeds a run-lock file for key at path, owned by a PID
// that is guaranteed not to be alive, so a subsequent `cuttings run`
// invocation's orphan sweep will find and clean it up. Returns the lock
// file's path.
func writeOrphanRunLock(t *testing.T, dir, key, path string) string {
	t.Helper()
	locksDir := filepath.Join(gitCommonDir(t, dir), "cuttings", "run-locks")
	if err := os.MkdirAll(locksDir, 0o750); err != nil {
		t.Fatalf("mkdir run-locks dir: %v", err)
	}

	lockPath := filepath.Join(locksDir, runLockFileName(key))
	lockJSON := fmt.Sprintf(`{"key":%q,"path":%q,"pid":%d,"createdAt":"2020-01-01T00:00:00Z"}`, key, path, deadPID(t))
	if err := os.WriteFile(lockPath, []byte(lockJSON), 0o600); err != nil {
		t.Fatalf("write run lock: %v", err)
	}
	return lockPath
}

// deadPID returns the PID of a process that has already exited, for tests that
// need a PID guaranteed not to be alive (orphan-detection fixtures).
//
// A reaped PID is immediately available for reuse, so the obvious version of
// this — run `true`, take its PID — can hand back a PID the OS has already
// recycled, which would make the orphan sweep skip the seeded lock. Confirm the
// PID really is gone, and try again if the kernel got there first.
func deadPID(t *testing.T) int {
	t.Helper()
	for attempt := 0; attempt < 10; attempt++ {
		cmd := exec.Command("true")
		if err := cmd.Run(); err != nil {
			t.Fatalf("run true: %v", err)
		}
		pid := cmd.Process.Pid
		// Signal 0 performs no action but reports whether the process exists.
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			return pid
		}
	}
	t.Fatal("could not obtain a PID that is definitely not in use")
	return 0
}

// signalTerminatedExitCode is the exec.ExitError.ExitCode() value Go reports
// for a process that was terminated by a signal rather than exiting normally.
const signalTerminatedExitCode = -1
