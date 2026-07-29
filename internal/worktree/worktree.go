/*
Copyright © 2026 Christoph Becker
*/

// Package worktree provides types for managing git worktrees as workstreams.
// Each workstream is stored in .worktrees/<branch-name>/ relative to the
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

// ErrWorktreeNotFound is returned when a workstream worktree does not exist.
var ErrWorktreeNotFound = errors.New("workstream not found")

// Worktree represents an active git worktree / workstream.
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

// Path returns the absolute filesystem path where a workstream worktree for
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
// preserved. Returns ErrWorktreeNotFound if no matching workstream exists.
func (m *Manager) Remove(branch string) error {
	path := m.Path(branch)

	// Verify the worktree actually exists before attempting removal.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return ErrWorktreeNotFound
	}

	//nolint:gosec // path is derived from an internal worktree directory, not raw user input.
	cmd := exec.Command("git", "worktree", "remove", path)
	cmd.Dir = m.repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree remove: %w\n%s", err, bytes.TrimSpace(out))
	}
	return nil
}

// Exists reports whether a workstream worktree for branch already exists.
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
