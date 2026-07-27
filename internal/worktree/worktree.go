/*
Copyright © 2026 Christoph Becker
*/

// Package worktree provides functions for managing git worktrees as workstreams.
// Each workstream is stored in .worktrees/<branch-name>/ relative to the
// repository root, enabling isolated working directories per branch.
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

// worktreesDir is the directory (relative to repo root) where worktrees are stored.
const worktreesDir = ".worktrees"

// Worktree represents an active git worktree / workstream.
type Worktree struct {
	// Branch is the branch name checked out in this worktree.
	Branch string
	// Path is the absolute filesystem path to the worktree directory.
	Path string
	// IsMain is true when this is the main worktree (the original repo clone).
	IsMain bool
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

// worktreePath returns the absolute path where a workstream worktree for the
// given branch is stored.
func worktreePath(repoRoot, branch string) string {
	// Replace slashes in branch names with OS path separators so that
	// "feature/foo" becomes ".worktrees/feature/foo" — a nested sub-directory.
	return filepath.Join(repoRoot, worktreesDir, filepath.FromSlash(branch))
}

// Add creates a new git worktree for branch at .worktrees/<branch>/. If
// createBranch is true the branch is created; otherwise it must already exist.
// Returns the absolute path of the new worktree.
func Add(repoRoot, branch string, createBranch bool) (string, error) {
	path := worktreePath(repoRoot, branch)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create worktree parent directory: %w", err)
	}

	var args []string
	if createBranch {
		// New branch: "git worktree add -b <branch> <path>"
		// Omit the commit-ish — it defaults to HEAD.
		args = []string{"worktree", "add", "-b", branch, path}
	} else {
		// Existing branch: "git worktree add <path> <branch>"
		args = []string{"worktree", "add", path, branch}
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git worktree add: %w\n%s", err, bytes.TrimSpace(out))
	}
	return path, nil
}

// List returns all git worktrees for the repository at repoRoot, including
// the main worktree and any additional worktrees under .worktrees/.
func List(repoRoot string) ([]Worktree, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}
	return parsePorcelain(repoRoot, out), nil
}

// parsePorcelain parses the output of `git worktree list --porcelain`.
// Each block is separated by a blank line and has the format:
//
//	worktree <path>
//	HEAD <sha>
//	branch refs/heads/<branch>   (or "detached")
func parsePorcelain(repoRoot string, data []byte) []Worktree {
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

// Remove removes the git worktree for the given branch. The branch itself is
// preserved. Returns ErrWorktreeNotFound if no matching workstream exists.
func Remove(repoRoot, branch string) error {
	path := worktreePath(repoRoot, branch)

	// Verify the worktree actually exists before attempting removal.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return ErrWorktreeNotFound
	}

	cmd := exec.Command("git", "worktree", "remove", path)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree remove: %w\n%s", err, bytes.TrimSpace(out))
	}
	return nil
}

// Exists reports whether a workstream worktree for branch already exists.
func Exists(repoRoot, branch string) bool {
	_, err := os.Stat(worktreePath(repoRoot, branch))
	return err == nil
}
