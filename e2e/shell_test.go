//go:build e2e

package e2e

import (
	"path/filepath"
	"testing"
)

func TestShell_Existing(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newCutting(t, h, "feature/foo")

	r := h.withEnv("SHELL", fakeShellPath()).run("shell", "feature/foo")
	requireExitCode(t, r, 0)

	wantPath := filepath.Join(dir, ".worktrees", "feature", "foo")
	requireContains(t, r.stdout, `Opening shell in cutting "feature/foo"`)
	requireContains(t, r.stdout, "CUTTING_BRANCH=feature/foo")
	requireContains(t, r.stdout, "CUTTING_PATH="+wantPath)
	requireContains(t, r.stdout, "PWD="+wantPath)
}

func TestShell_NotFound(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)

	r := h.run("shell", "nope")
	requireExitCode(t, r, 1)
	requireContains(t, r.stderr, "no cutting found")
	requireContains(t, r.stderr, "new nope")
}
