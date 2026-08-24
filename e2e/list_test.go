//go:build e2e

package e2e

import (
	"path/filepath"
	"testing"
)

func TestList_MainOnly(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)

	r := h.run("list")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "BRANCH")
	requireContains(t, r.stdout, "main")
	requireNotContains(t, r.stdout, "workstream")
}

func TestList_WithWorkstreams(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newWorkstream(t, h, "feature/a")
	newWorkstream(t, h, "feature/b")

	r := h.run("list")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "feature/a")
	requireContains(t, r.stdout, "feature/b")
	requireContains(t, r.stdout, "main")

	wantPathA := filepath.Join(dir, ".worktrees", "feature", "a")
	requireContains(t, r.stdout, wantPathA)
}

func TestList_AliasParity(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newWorkstream(t, h, "feature/a")

	list := h.run("list")
	ls := h.run("ls")
	requireExitCode(t, list, 0)
	requireExitCode(t, ls, 0)
	if list.stdout != ls.stdout {
		t.Fatalf("list and ls output differ:\nlist:\n%s\nls:\n%s", list.stdout, ls.stdout)
	}
}
