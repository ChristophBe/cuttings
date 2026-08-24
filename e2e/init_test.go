//go:build e2e

package e2e

import "testing"

func TestInit_Fresh(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)

	r := h.run("init")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "Created")

	content := readConfigFile(t, dir)
	requireContains(t, content, "worktrees_dir: .worktrees")
	requireContains(t, content, `default_branch: ""`)
	requireContains(t, content, "run_cleanup_on_signal: true")
}

func TestInit_AlreadyExists_NoOverwrite(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	requireExitCode(t, h.run("init"), 0)

	before := readConfigFile(t, dir)

	r := h.run("init")
	requireExitCode(t, r, 1)
	requireContains(t, r.stderr, "already exists")

	after := readConfigFile(t, dir)
	if before != after {
		t.Fatalf("config file content changed despite missing --overwrite")
	}
}

func TestInit_Overwrite(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	requireExitCode(t, h.run("init"), 0)

	r := h.run("init", "--overwrite")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "Created")
}

func TestInit_OutsideRepo(t *testing.T) {
	dir := realPath(t, t.TempDir())
	h := newHarness(t, dir)

	r := h.run("init")
	requireExitCode(t, r, 1)
	requireContains(t, r.stderr, "not a git repository")
}

// TestInit_ShorthandOverwrite verifies -o is equivalent to --overwrite.
func TestInit_ShorthandOverwrite(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	requireExitCode(t, h.run("init"), 0)

	r := h.run("init", "-o")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "Created")
}
