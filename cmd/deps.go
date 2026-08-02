/*
Copyright © 2026 Christoph Becker
*/

package cmd

import (
	"github.com/ChristophBe/workstreams/internal/config"
	"github.com/ChristophBe/workstreams/internal/worktree"
)

// WorktreeManager abstracts git worktree operations scoped to a repository.
// All methods are scoped to the repository root and worktrees directory
// established at construction time. It is satisfied by *worktree.Manager.
type WorktreeManager interface {
	Add(branch string, createBranch bool, base string) (string, error)
	AddDetached(name, base string) (string, error)
	CurrentBranch() (string, error)
	ListBranches() ([]string, error)
	List() ([]worktree.Worktree, error)
	Remove(branch string) error
	Exists(branch string) bool
	BranchExists(branch string) bool
	Path(branch string) string
}

// ShellSpawner abstracts interactive shell spawning for the cmd layer.
// It is satisfied by *shell.Spawner.
type ShellSpawner interface {
	Spawn(dir, branch string) error
}

// CommandRunner abstracts non-interactive command execution for the cmd layer.
// It is satisfied by *shell.Spawner.
type CommandRunner interface {
	Run(dir, branch string, command []string) error
}

// deps holds the resolved dependencies for the current command invocation.
// Populated once by PersistentPreRunE in root.go; read by all subcommands.
var deps struct {
	cfg     *config.Config
	wt      WorktreeManager
	spawner ShellSpawner
	runner  CommandRunner
}
