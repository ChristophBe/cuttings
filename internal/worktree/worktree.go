/*
Copyright © 2026 Christoph Becker
*/

// Package worktree provides types for managing git worktrees as cuttings.
// Each cutting is stored in .worktrees/<branch-name>/ relative to the
// repository root, enabling isolated working directories per branch.
//
// Usage:
//
//	m := worktree.NewManager(repoRoot, ".worktrees")
//	path, err := m.Add("feature/foo", true, "")
package worktree

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// ErrNotAGitRepo is returned when no git repository can be found.
var ErrNotAGitRepo = errors.New("not a git repository (or any of the parent directories)")

// ErrWorktreeNotFound is returned when a cutting worktree does not exist.
var ErrWorktreeNotFound = errors.New("cutting not found")

// Worktree represents an active git worktree / cutting.
type Worktree struct {
	// Branch is the branch name checked out in this worktree.
	Branch string
	// Path is the absolute filesystem path to the worktree directory.
	Path string
	// IsMain is true when this is the main worktree (the original repo clone).
	IsMain bool
}

// Manager provides git worktree operations scoped to a single repository.
// Construct one with NewManager; all methods are then called without repeating
// the repository root or worktrees directory.
type Manager struct {
	repoRoot     string
	worktreesDir string
}

// NewManager returns a Manager for the repository at repoRoot, storing
// worktrees under worktreesDir (relative to repoRoot).
func NewManager(repoRoot, worktreesDir string) *Manager {
	return &Manager{repoRoot: repoRoot, worktreesDir: worktreesDir}
}

// Path returns the absolute filesystem path where a cutting worktree for
// branch is stored. The path may not exist yet.
func (m *Manager) Path(branch string) string {
	// Replace slashes in branch names with OS path separators so that
	// "feature/foo" becomes ".worktrees/feature/foo" — a nested sub-directory.
	return filepath.Join(m.repoRoot, m.worktreesDir, filepath.FromSlash(branch))
}

// Add creates a new git worktree for branch. If createBranch is true the
// branch is created; otherwise it must already exist. base optionally
// specifies the commit-ish to fork from when creating a new branch (e.g.
// "main", "origin/develop"). An empty string defaults to HEAD.
// Returns the absolute path of the new worktree.
func (m *Manager) Add(branch string, createBranch bool, base string) (string, error) {
	path := m.Path(branch)

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil { //nolint:gosec // 0750: group-readable worktree dirs are fine
		return "", fmt.Errorf("create worktree parent directory: %w", err)
	}

	var args []string
	if createBranch {
		// New branch: "git worktree add -b <branch> <path> [<base>]"
		// Omit the commit-ish when empty — it defaults to HEAD.
		args = []string{"worktree", "add", "-b", branch, path}
		if base != "" {
			args = append(args, base)
		}
	} else {
		// Existing branch: "git worktree add <path> <branch>"
		args = []string{"worktree", "add", path, branch}
	}

	//nolint:gosec // git args include user-supplied branch names; this is the tool's purpose.
	cmd := exec.Command("git", args...)
	cmd.Dir = m.repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git worktree add: %w\n%s", err, bytes.TrimSpace(out))
	}
	return path, nil
}

// List returns all git worktrees for the repository, including the main
// worktree and any additional worktrees under the configured worktrees directory.
func (m *Manager) List() ([]Worktree, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = m.repoRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}
	return parsePorcelain(out), nil
}

// Remove removes the git worktree for the given branch. The branch itself is
// preserved. Returns ErrWorktreeNotFound if no matching cutting exists.
// By default git refuses to remove a worktree with uncommitted or untracked
// changes; pass force to bypass that check.
func (m *Manager) Remove(branch string, force bool) error {
	path := m.Path(branch)

	// Verify the worktree actually exists before attempting removal.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return ErrWorktreeNotFound
	}

	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)

	//nolint:gosec // path is derived from an internal worktree directory, not raw user input.
	cmd := exec.Command("git", args...)
	cmd.Dir = m.repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree remove: %w\n%s", err, bytes.TrimSpace(out))
	}
	return nil
}

// Exists reports whether a cutting worktree for branch already exists.
func (m *Manager) Exists(branch string) bool {
	_, err := os.Stat(m.Path(branch))
	return err == nil
}

// BranchExists reports whether branch already exists in the repository
// (checking both local and remote tracking refs).
func (m *Manager) BranchExists(branch string) bool {
	//nolint:gosec // branch is a user-supplied git ref; this is intentional.
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	cmd.Dir = m.repoRoot
	if cmd.Run() == nil {
		return true
	}
	// Also check remote tracking branches.
	//nolint:gosec // branch is a user-supplied git ref; this is intentional.
	cmd2 := exec.Command("git", "show-ref", "--verify", "--quiet",
		"refs/remotes/origin/"+strings.TrimPrefix(branch, "origin/"))
	cmd2.Dir = m.repoRoot
	return cmd2.Run() == nil
}

// ListBranches returns the names of all local branches in the repository.
func (m *Manager) ListBranches() ([]string, error) {
	cmd := exec.Command("git", "branch", "--format=%(refname:short)")
	cmd.Dir = m.repoRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git branch: %w", err)
	}
	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			branches = append(branches, line)
		}
	}
	return branches, nil
}

// ListMergedBranches returns the names of local branches that are fully
// merged into base — i.e. base already contains every commit on that
// branch. base itself is always included, since a branch is trivially
// merged into itself.
func (m *Manager) ListMergedBranches(base string) ([]string, error) {
	//nolint:gosec // base is a user-supplied git ref; this is intentional.
	cmd := exec.Command("git", "branch", "--format=%(refname:short)", "--merged", base)
	cmd.Dir = m.repoRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git branch --merged: %w", err)
	}
	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			branches = append(branches, line)
		}
	}
	return branches, nil
}

// CurrentBranch returns the name of the branch currently checked out in the
// main worktree. Returns "HEAD" if the repository is in detached HEAD state.
func (m *Manager) CurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = m.repoRoot
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --abbrev-ref HEAD: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// AddDetached creates a new git worktree in detached HEAD state. name is used
// only to derive the worktree directory path; no branch is created. base
// optionally specifies a commit-ish to check out (defaults to HEAD when empty).
func (m *Manager) AddDetached(name, base string) (string, error) {
	path := m.Path(name)

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil { //nolint:gosec // 0750: group-readable worktree dirs are fine
		return "", fmt.Errorf("create worktree parent directory: %w", err)
	}

	args := []string{"worktree", "add", "--detach", path}
	if base != "" {
		args = append(args, base)
	}

	//nolint:gosec // args include user-supplied values; this is the tool's purpose.
	cmd := exec.Command("git", args...)
	cmd.Dir = m.repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git worktree add --detach: %w\n%s", err, bytes.TrimSpace(out))
	}
	return path, nil
}

// RunLock records an in-progress `cuttings run` invocation so that a
// SweepOrphans call on a later invocation can detect and clean up a
// worktree left behind by a parent process that died uncatchably (SIGKILL,
// crash, `kill -9`) before its own deferred cleanup could run.
type RunLock struct {
	Key       string    `json:"key"`
	Path      string    `json:"path"`
	PID       int       `json:"pid"`
	CreatedAt time.Time `json:"createdAt"`
}

// gitCommonDir returns the absolute path to the repository's shared .git
// directory, resolving linked-worktree ".git" files to the common dir they
// point at. This ensures per-repo state like run locks lands in one place
// regardless of which worktree the command was invoked from.
func (m *Manager) gitCommonDir() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--path-format=absolute", "--git-common-dir")
	cmd.Dir = m.repoRoot
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --git-common-dir: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// runLocksDir returns the directory where run lock files are stored.
func (m *Manager) runLocksDir() (string, error) {
	commonDir, err := m.gitCommonDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(commonDir, "cuttings", "run-locks"), nil
}

// lockFileName derives a filesystem-safe, deterministic file name for key
// (which may contain "/" for branch names). The real key is stored in the
// lock file's JSON body.
func lockFileName(key string) string {
	sum := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%x.json", sum[:8])
}

// Lock records that a run for key (a worktree key, e.g. "cut-run-<ts>" or a
// branch name) is in progress, owned by the current process. It must be
// paired with a later Unlock call. Lock is best-effort infrastructure for
// orphan detection: a caller should log a failure here rather than abort the
// run, since the worst case is that SweepOrphans simply can't find this run
// later.
func (m *Manager) Lock(key string) error {
	dir, err := m.runLocksDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o750); err != nil { //nolint:gosec // 0750: group-readable, matches worktree dirs.
		return fmt.Errorf("create run-locks directory: %w", err)
	}

	lock := RunLock{
		Key:       key,
		Path:      m.Path(key),
		PID:       os.Getpid(),
		CreatedAt: time.Now(),
	}
	data, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal run lock: %w", err)
	}

	if err := os.WriteFile(filepath.Join(dir, lockFileName(key)), data, 0o600); err != nil {
		return fmt.Errorf("write run lock: %w", err)
	}
	return nil
}

// Unlock removes the lock file previously written by Lock for key. A missing
// lock file is not an error.
func (m *Manager) Unlock(key string) error {
	dir, err := m.runLocksDir()
	if err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(dir, lockFileName(key))); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove run lock: %w", err)
	}
	return nil
}

// SweepOrphans finds run locks left behind by cuttings processes that
// died without running their own cleanup (e.g. killed with SIGKILL, or
// crashed) — the owning PID is no longer alive. For each such orphan it
// removes the associated worktree and the stale lock file, returning the
// keys it cleaned up. A lock whose recorded PID is still alive is left
// untouched. Errors removing an individual orphan are collected and
// returned via errors.Join without stopping the sweep of remaining entries;
// affected lock files are left in place so a later sweep can retry them.
func (m *Manager) SweepOrphans() ([]string, error) {
	dir, err := m.runLocksDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read run-locks directory: %w", err)
	}

	var cleaned []string
	var errs []error
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		lockPath := filepath.Join(dir, entry.Name())

		//nolint:gosec // path built from a directory entry we just listed.
		data, err := os.ReadFile(lockPath)
		if err != nil {
			errs = append(errs, fmt.Errorf("read lock %s: %w", entry.Name(), err))
			continue
		}
		var lock RunLock
		if err := json.Unmarshal(data, &lock); err != nil {
			// Corrupt/foreign file — remove it so it doesn't wedge future sweeps.
			_ = os.Remove(lockPath)
			continue
		}
		if processAlive(lock.PID) {
			continue
		}

		if removeErr := m.Remove(lock.Key, false); removeErr != nil && !errors.Is(removeErr, ErrWorktreeNotFound) {
			errs = append(errs, fmt.Errorf("remove orphaned cutting %q: %w", lock.Key, removeErr))
			continue
		}
		if err := os.Remove(lockPath); err != nil && !os.IsNotExist(err) {
			errs = append(errs, fmt.Errorf("remove stale lock for %q: %w", lock.Key, err))
			continue
		}
		cleaned = append(cleaned, lock.Key)
	}
	return cleaned, errors.Join(errs...)
}

// processAlive reports whether a process with the given PID currently
// exists. Unix-only (this codebase already assumes Unix via syscall.Exec in
// package shell). Sending signal 0 performs no action but still returns an
// error (typically ESRCH) if the process does not exist.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid) // on Unix this always succeeds
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// FindRepoRoot walks up the directory tree from the current working directory
// to find the root of the git repository (the directory containing .git).
func FindRepoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", ErrNotAGitRepo
	}
	return strings.TrimSpace(string(out)), nil
}

// parsePorcelain parses the output of `git worktree list --porcelain`.
// Each block is separated by a blank line and has the format:
//
//	worktree <path>
//	HEAD <sha>
//	branch refs/heads/<branch>   (or "detached")
func parsePorcelain(data []byte) []Worktree {
	var result []Worktree
	var current Worktree
	isFirst := true

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "worktree "):
			current = Worktree{Path: strings.TrimPrefix(line, "worktree "), IsMain: isFirst}
			isFirst = false
		case strings.HasPrefix(line, "branch "):
			ref := strings.TrimPrefix(line, "branch ")
			current.Branch = strings.TrimPrefix(ref, "refs/heads/")
		case line == "":
			if current.Path != "" {
				result = append(result, current)
				current = Worktree{}
			}
		}
	}
	// Flush last block (no trailing blank line in some git versions)
	if current.Path != "" {
		result = append(result, current)
	}
	return result
}
