/*
Copyright © 2026 Christoph Becker
*/

// Package shell provides functionality for spawning an interactive shell
// session inside a workstream directory. The spawned shell replaces the
// current process via syscall.Exec, so exiting the shell returns the user
// to their original terminal naturally.
package shell

import (
	"fmt"
	"os"
	"strings"
	"syscall"
)

// Spawner provides shell-spawning functionality for workstream directories.
// It is a zero-value-usable struct; use NewSpawner to construct one explicitly.
type Spawner struct{}

// NewSpawner returns a Spawner ready for use.
func NewSpawner() *Spawner {
	return &Spawner{}
}

// Spawn starts an interactive shell in dir by replacing the current process
// (using syscall.Exec). The shell binary is taken from the SHELL environment
// variable, falling back to /bin/sh when unset.
//
// Two additional environment variables are injected into the shell:
//   - WORKSTREAM_BRANCH: the branch name of the workstream
//   - WORKSTREAM_PATH:   the absolute path to the worktree directory
//
// Because Exec replaces the current process, this function only returns on
// error.
func (s *Spawner) Spawn(dir, branch string) error {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	env := BuildEnv(branch, dir)

	if err := os.Chdir(dir); err != nil {
		return fmt.Errorf("change directory to workstream: %w", err)
	}

	//nolint:gosec // $SHELL is the user's own choice of shell — intentional.
	return syscall.Exec(shell, []string{shell}, env)
}

// BuildEnv returns the current environment with WORKSTREAM_BRANCH and
// WORKSTREAM_PATH set (or overwritten). Existing values for these keys are
// replaced so that nested workstreams always reflect the innermost context.
func BuildEnv(branch, path string) []string {
	current := os.Environ()
	out := make([]string, 0, len(current)+2)

	for _, e := range current {
		if strings.HasPrefix(e, "WORKSTREAM_BRANCH=") || strings.HasPrefix(e, "WORKSTREAM_PATH=") {
			continue
		}
		out = append(out, e)
	}
	out = append(out,
		"WORKSTREAM_BRANCH="+branch,
		"WORKSTREAM_PATH="+path,
	)
	return out
}
