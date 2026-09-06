/*
Copyright © 2026 Christoph Becker
*/

package cmd

import (
	"context"

	"github.com/ChristophBe/cuttings/internal/config"
	"github.com/ChristophBe/cuttings/internal/run"
	"github.com/ChristophBe/cuttings/internal/worktree"
)

// The interfaces below are deliberately narrow: each names one role a command
// needs, so a command's tests stub only the calls it actually makes rather
// than the whole worktree surface. WorktreeManager composes them and is what
// the wiring in root.go supplies; *worktree.Manager satisfies all of them.
type (
	// cuttingLister lists the worktrees of the repository.
	cuttingLister interface {
		List() ([]worktree.Worktree, error)
	}

	// branchLister lists branches — all of them, or only those merged into a base.
	branchLister interface {
		ListBranches() ([]string, error)
		ListMergedBranches(base string) ([]string, error)
		CurrentBranch() (string, error)
	}

	// cuttingRemover removes a single cutting's worktree.
	cuttingRemover interface {
		Remove(branch string, force bool) error
	}

	// cuttingProvisioner creates cutting worktrees and answers questions about
	// which ones already exist.
	cuttingProvisioner interface {
		Add(branch string, createBranch bool, base string) (string, error)
		AddDetached(name, base string) (string, error)
		Exists(branch string) bool
		BranchExists(branch string) bool
		Path(branch string) string
	}
)

// WorktreeManager abstracts git worktree operations scoped to a repository.
// All methods are scoped to the repository root and worktrees directory
// established at construction time. It is satisfied by *worktree.Manager.
type WorktreeManager interface {
	cuttingLister
	branchLister
	cuttingProvisioner
	cuttingRemover
}

// ShellSpawner abstracts interactive shell spawning for the cmd layer.
// It is satisfied by *shell.Spawner.
type ShellSpawner interface {
	Spawn(dir, branch string) error
}

// CommandRunner abstracts non-interactive command execution for the cmd layer.
// It is satisfied by *shell.Spawner.
type CommandRunner interface {
	Run(ctx context.Context, dir, branch string, command []string) error
}

// Deps holds the dependencies of a command tree. One Deps is created per
// process by Execute and populated once by the root command's
// PersistentPreRunE, which is the single wiring point; commands close over the
// pointer and read its fields when they run. Tests build their own Deps with
// test doubles instead of going through PersistentPreRunE.
type Deps struct {
	cfg     *config.Config
	wt      WorktreeManager
	locks   run.Locks
	spawner ShellSpawner
	runner  CommandRunner
}
