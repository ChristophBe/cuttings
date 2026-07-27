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
	"syscall"
)

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
func Spawn(dir, branch string) error {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	env := buildEnv(branch, dir)

	if err := os.Chdir(dir); err != nil {
		return fmt.Errorf("change directory to workstream: %w", err)
	}

	return syscall.Exec(shell, []string{shell}, env) //nolint:wrapcheck
}

// buildEnv returns the current environment with WORKSTREAM_BRANCH and
// WORKSTREAM_PATH set (or overwritten). Existing values for these keys are
// replaced so that nested workstreams always reflect the innermost context.
func buildEnv(branch, path string) []string {
	current := os.Environ()
	out := make([]string, 0, len(current)+2)

	for _, e := range current {
		if len(e) >= 20 && e[:20] == "WORKSTREAM_BRANCH=" {
			continue
		}
		if len(e) >= 17 && e[:17] == "WORKSTREAM_PATH=" {
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
