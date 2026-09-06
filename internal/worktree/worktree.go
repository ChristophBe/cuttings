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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	// runner executes git. A field so tests can assert on the arguments a
	// method builds without creating a real repository; nil means the real
	// git binary.
	runner gitRunner
}

// gitRunner executes git with args in dir. combined selects CombinedOutput
// (stdout merged with stderr, for the commands whose diagnostics the caller
// surfaces to the user) over plain Output.
type gitRunner func(dir string, combined bool, args ...string) ([]byte, error)

// NewManager returns a Manager for the repository at repoRoot, storing
// worktrees under worktreesDir (relative to repoRoot).
func NewManager(repoRoot, worktreesDir string) *Manager {
	return &Manager{repoRoot: repoRoot, worktreesDir: worktreesDir}
}

// git runs git with args in the repository root and returns its stdout.
func (m *Manager) git(args ...string) ([]byte, error) {
	return m.exec(false, args...)
}

// gitCombined runs git with args in the repository root and returns stdout and
// stderr together, so a failure message from git can be shown to the user.
func (m *Manager) gitCombined(args ...string) ([]byte, error) {
	return m.exec(true, args...)
}

func (m *Manager) exec(combined bool, args ...string) ([]byte, error) {
	if m.runner != nil {
		return m.runner(m.repoRoot, combined, args...)
	}
	//nolint:gosec // git args include user-supplied branch names and refs; this is the tool's purpose.
	cmd := exec.Command("git", args...)
	cmd.Dir = m.repoRoot
	if combined {
		return cmd.CombinedOutput()
	}
	return cmd.Output()
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

	out, err := m.gitCombined(args...)
	if err != nil {
		return "", fmt.Errorf("git worktree add: %w\n%s", err, bytes.TrimSpace(out))
	}
	return path, nil
}

// List returns all git worktrees for the repository, including the main
// worktree and any additional worktrees under the configured worktrees directory.
func (m *Manager) List() ([]Worktree, error) {
	out, err := m.git("worktree", "list", "--porcelain")
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

	out, err := m.gitCombined(args...)
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
	if _, err := m.git("show-ref", "--verify", "--quiet", "refs/heads/"+branch); err == nil {
		return true
	}
	// Also check remote tracking branches.
	_, err := m.git("show-ref", "--verify", "--quiet",
		"refs/remotes/origin/"+strings.TrimPrefix(branch, "origin/"))
	return err == nil
}

// ListBranches returns the names of all local branches in the repository.
func (m *Manager) ListBranches() ([]string, error) {
	out, err := m.git("branch", "--format=%(refname:short)")
	if err != nil {
		return nil, fmt.Errorf("git branch: %w", err)
	}
	return parseBranchList(out), nil
}

// ListMergedBranches returns the names of local branches that are fully
// merged into base — i.e. base already contains every commit on that
// branch. base itself is always included, since a branch is trivially
// merged into itself.
func (m *Manager) ListMergedBranches(base string) ([]string, error) {
	out, err := m.git("branch", "--format=%(refname:short)", "--merged", base)
	if err != nil {
		return nil, fmt.Errorf("git branch --merged: %w", err)
	}
	return parseBranchList(out), nil
}

// CurrentBranch returns the name of the branch currently checked out in the
// main worktree. Returns "HEAD" if the repository is in detached HEAD state.
func (m *Manager) CurrentBranch() (string, error) {
	out, err := m.git("rev-parse", "--abbrev-ref", "HEAD")
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

	out, err := m.gitCombined(args...)
	if err != nil {
		return "", fmt.Errorf("git worktree add --detach: %w\n%s", err, bytes.TrimSpace(out))
	}
	return path, nil
}

// GitCommonDir returns the absolute path to the repository's shared .git
// directory, resolving linked-worktree ".git" files to the common dir they
// point at. This lets per-repo state (such as run locks) land in one place
// regardless of which worktree the command was invoked from.
func (m *Manager) GitCommonDir() (string, error) {
	out, err := m.git("rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("git rev-parse --git-common-dir: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
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

// parseBranchList parses the output of a `git branch --format=%(refname:short)`
// invocation into a slice of branch names, dropping blank lines.
func parseBranchList(out []byte) []string {
	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			branches = append(branches, line)
		}
	}
	return branches
}

// Cuttings filters trees down to the actual cuttings: everything except the
// main worktree and any entry without a branch (e.g. a detached-HEAD worktree
// created by `cuttings run`, which is temporary and not user-addressable).
func Cuttings(trees []Worktree) []Worktree {
	var out []Worktree
	for _, t := range trees {
		if t.IsMain || t.Branch == "" {
			continue
		}
		out = append(out, t)
	}
	return out
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
