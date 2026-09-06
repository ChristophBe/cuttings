/*
Copyright © 2026 Christoph Becker
*/
package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/ChristophBe/cuttings/internal/config"
	"github.com/ChristophBe/cuttings/internal/worktree"
)

// cmdFixture runs one command against test doubles. Like runFixture it owns
// its Deps and its buffers, so nothing is shared between tests.
type cmdFixture struct {
	deps    *Deps
	wt      *mockWorktreeManager
	spawner *mockSpawner
	stdout  bytes.Buffer
	stderr  bytes.Buffer
}

func newCmdFixture(wt *mockWorktreeManager) *cmdFixture {
	spawner := &mockSpawner{}
	return &cmdFixture{
		deps: &Deps{
			cfg:     &config.Config{},
			wt:      wt,
			spawner: spawner,
		},
		wt:      wt,
		spawner: spawner,
	}
}

// exec runs the command built by build with args, going through Cobra's real
// flag parsing and Args validation.
func (f *cmdFixture) exec(build func(*Deps) *cobra.Command, args ...string) error {
	cmd := build(f.deps)
	cmd.SetOut(&f.stdout)
	cmd.SetErr(&f.stderr)
	cmd.SetArgs(args)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	return cmd.Execute()
}

func (f *cmdFixture) out() string { return f.stdout.String() }

// cuttings builds a worktree listing: the main worktree followed by one
// cutting per branch.
func cuttings(branches ...string) []worktree.Worktree {
	trees := []worktree.Worktree{{Branch: "main", Path: "/repo", IsMain: true}}
	for _, b := range branches {
		trees = append(trees, worktree.Worktree{Branch: b, Path: "/repo/.worktrees/" + b})
	}
	return trees
}

// --- new ---

func TestNewCmd_CreatesWorktreeAndSpawnsShell(t *testing.T) {
	t.Parallel()

	f := newCmdFixture(&mockWorktreeManager{addPath: "/repo/.worktrees/feature/foo"})

	if err := f.exec(newNewCmd, "feature/foo"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if f.wt.addBranch != "feature/foo" {
		t.Errorf("Add branch = %q, want %q", f.wt.addBranch, "feature/foo")
	}
	if !f.wt.addCreateBranch {
		t.Error("Add should create the branch when it does not exist yet")
	}
	if !f.spawner.spawnCalled {
		t.Error("Spawn() was not called")
	}
	if f.spawner.spawnDir != "/repo/.worktrees/feature/foo" {
		t.Errorf("Spawn dir = %q, want the path returned by Add", f.spawner.spawnDir)
	}
	if f.spawner.spawnBranch != "feature/foo" {
		t.Errorf("Spawn branch = %q, want %q", f.spawner.spawnBranch, "feature/foo")
	}
	if !strings.Contains(f.out(), `Creating cutting for branch "feature/foo"`) {
		t.Errorf("stdout = %q, want the creation message", f.out())
	}
}

func TestNewCmd_ExistingBranch_DoesNotRecreateIt(t *testing.T) {
	t.Parallel()

	f := newCmdFixture(&mockWorktreeManager{addPath: "/repo/.worktrees/foo", branchExists: true})

	if err := f.exec(newNewCmd, "foo"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.wt.addCreateBranch {
		t.Error("Add should not create a branch that already exists")
	}
}

func TestNewCmd_CuttingAlreadyExists(t *testing.T) {
	t.Parallel()

	f := newCmdFixture(&mockWorktreeManager{existsResult: true})

	err := f.exec(newNewCmd, "feature/foo")
	if err == nil {
		t.Fatal("expected an error when the cutting already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want it to say the cutting already exists", err)
	}
	if f.spawner.spawnCalled {
		t.Error("Spawn() should not be called when the cutting already exists")
	}
}

func TestNewCmd_SourceFlagAndConfigDefault(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		defaultBranch string
		args          []string
		wantBase      string
	}{
		{name: "--source wins", defaultBranch: "develop", args: []string{"foo", "--source", "main"}, wantBase: "main"},
		{name: "falls back to config", defaultBranch: "develop", args: []string{"foo"}, wantBase: "develop"},
		{name: "empty means HEAD", args: []string{"foo"}, wantBase: ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			f := newCmdFixture(&mockWorktreeManager{addPath: "/repo/.worktrees/foo"})
			f.deps.cfg = &config.Config{DefaultBranch: c.defaultBranch}

			if err := f.exec(newNewCmd, c.args...); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if f.wt.addBase != c.wantBase {
				t.Errorf("Add base = %q, want %q", f.wt.addBase, c.wantBase)
			}
		})
	}
}

func TestNewCmd_AddFails(t *testing.T) {
	t.Parallel()

	f := newCmdFixture(&mockWorktreeManager{addErr: errors.New("git exploded")})

	err := f.exec(newNewCmd, "foo")
	if err == nil {
		t.Fatal("expected an error when Add fails")
	}
	if !strings.Contains(err.Error(), "create cutting") {
		t.Errorf("error = %q, want the 'create cutting' prefix", err)
	}
	if f.spawner.spawnCalled {
		t.Error("Spawn() should not be called when Add fails")
	}
}

func TestNewCmd_RequiresExactlyOneBranch(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{}, {"a", "b"}} {
		f := newCmdFixture(&mockWorktreeManager{})
		if err := f.exec(newNewCmd, args...); err == nil {
			t.Errorf("args %v: expected an argument-validation error", args)
		}
	}
}

// --- shell ---

func TestShellCmd_SpawnsInExistingCutting(t *testing.T) {
	t.Parallel()

	f := newCmdFixture(&mockWorktreeManager{existsResult: true, pathResult: "/repo/.worktrees/foo"})

	if err := f.exec(newShellCmd, "foo"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.spawner.spawnDir != "/repo/.worktrees/foo" {
		t.Errorf("Spawn dir = %q, want the cutting path", f.spawner.spawnDir)
	}
	if f.spawner.spawnBranch != "foo" {
		t.Errorf("Spawn branch = %q, want %q", f.spawner.spawnBranch, "foo")
	}
}

func TestShellCmd_NotFound(t *testing.T) {
	t.Parallel()

	f := newCmdFixture(&mockWorktreeManager{existsResult: false})

	err := f.exec(newShellCmd, "foo")
	if err == nil {
		t.Fatal("expected an error when no cutting exists")
	}
	if !strings.Contains(err.Error(), "no cutting found") {
		t.Errorf("error = %q, want 'no cutting found'", err)
	}
	if f.spawner.spawnCalled {
		t.Error("Spawn() should not be called for a missing cutting")
	}
}

// --- list ---

func TestListCmd_MarksMainAndCuttings(t *testing.T) {
	t.Parallel()

	f := newCmdFixture(&mockWorktreeManager{listResult: cuttings("feature/foo")})

	if err := f.exec(newListCmd); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := f.out()
	for _, want := range []string{"BRANCH", "main", "feature/foo", "cutting"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	// The main worktree's row must be typed "main", not "cutting".
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "main ") && !strings.HasSuffix(strings.TrimSpace(line), "main") {
			t.Errorf("main worktree row typed incorrectly: %q", line)
		}
	}
}

func TestListCmd_Error(t *testing.T) {
	t.Parallel()

	f := newCmdFixture(&mockWorktreeManager{listErr: errors.New("git failed")})
	if err := f.exec(newListCmd); err == nil {
		t.Fatal("expected the List error to propagate")
	}
}

// --- remove ---

func TestRemoveCmd_SingleBranch(t *testing.T) {
	t.Parallel()

	f := newCmdFixture(&mockWorktreeManager{})

	if err := f.exec(newRemoveCmd, "feature/foo"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.wt.removeKey != "feature/foo" {
		t.Errorf("Remove branch = %q, want %q", f.wt.removeKey, "feature/foo")
	}
	if !strings.Contains(f.out(), "(branch preserved)") {
		t.Errorf("output should say the branch is preserved:\n%s", f.out())
	}
}

func TestRemoveCmd_ForceFlagPassedThrough(t *testing.T) {
	t.Parallel()

	f := newCmdFixture(&mockWorktreeManager{})
	if err := f.exec(newRemoveCmd, "foo", "--force"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.wt.removeForces) != 1 || !f.wt.removeForces[0] {
		t.Errorf("force = %v, want [true]", f.wt.removeForces)
	}
}

// A missing worktree must be reported in cuttings' own vocabulary rather than
// leaking the internal sentinel error.
func TestRemoveCmd_NotFoundIsTranslated(t *testing.T) {
	t.Parallel()

	f := newCmdFixture(&mockWorktreeManager{removeErr: worktree.ErrWorktreeNotFound})

	err := f.exec(newRemoveCmd, "feature/foo")
	if err == nil {
		t.Fatal("expected an error for a missing cutting")
	}
	if !strings.Contains(err.Error(), `no cutting found for branch "feature/foo"`) {
		t.Errorf("error = %q, want the friendly not-found message", err)
	}
}

func TestRemoveCmd_DryRun_RemovesNothing(t *testing.T) {
	t.Parallel()

	f := newCmdFixture(&mockWorktreeManager{existsResult: true})

	if err := f.exec(newRemoveCmd, "feature/foo", "--dry-run"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.wt.removeCalled {
		t.Error("--dry-run must not remove anything")
	}
	if !strings.Contains(f.out(), `Would remove cutting for "feature/foo".`) {
		t.Errorf("output = %q, want the dry-run message", f.out())
	}
}

func TestRemoveCmd_DryRun_MissingCutting(t *testing.T) {
	t.Parallel()

	f := newCmdFixture(&mockWorktreeManager{existsResult: false})
	if err := f.exec(newRemoveCmd, "feature/foo", "--dry-run"); err == nil {
		t.Fatal("expected an error when the cutting does not exist")
	}
}

func TestRemoveCmd_All(t *testing.T) {
	t.Parallel()

	f := newCmdFixture(&mockWorktreeManager{listResult: cuttings("feature/a", "feature/b")})

	if err := f.exec(newRemoveCmd, "--all"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.wt.removedBranches) != 2 {
		t.Fatalf("removed = %v, want both cuttings", f.wt.removedBranches)
	}
	// The main worktree must never be a candidate.
	for _, b := range f.wt.removedBranches {
		if b == "main" {
			t.Error("--all must not remove the main worktree")
		}
	}
}

func TestRemoveCmd_All_RejectsABranchArgument(t *testing.T) {
	t.Parallel()

	f := newCmdFixture(&mockWorktreeManager{listResult: cuttings("feature/a")})
	if err := f.exec(newRemoveCmd, "--all", "feature/a"); err == nil {
		t.Fatal("expected --all to reject a positional branch argument")
	}
}

func TestRemoveCmd_All_NoCuttings(t *testing.T) {
	t.Parallel()

	f := newCmdFixture(&mockWorktreeManager{listResult: cuttings()})

	if err := f.exec(newRemoveCmd, "--all"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(f.out(), "No cuttings to remove.") {
		t.Errorf("output = %q, want the empty-case message", f.out())
	}
}

// One cutting failing (e.g. uncommitted changes) must not stop the others.
func TestRemoveCmd_All_PartialFailure(t *testing.T) {
	t.Parallel()

	f := newCmdFixture(&mockWorktreeManager{
		listResult: cuttings("feature/a", "feature/b", "feature/c"),
		removeErrs: map[string]error{"feature/b": errors.New("uncommitted changes")},
	})

	err := f.exec(newRemoveCmd, "--all")
	if err == nil {
		t.Fatal("expected the failing cutting to be reported")
	}
	if len(f.wt.removedBranches) != 3 {
		t.Errorf("removed = %v, want all three attempted", f.wt.removedBranches)
	}
}

// --- prune ---

func TestPruneCmd_RemovesOnlyMergedCuttings(t *testing.T) {
	t.Parallel()

	f := newCmdFixture(&mockWorktreeManager{
		listResult:   cuttings("feature/merged", "feature/unmerged"),
		mergedResult: []string{"main", "feature/merged"},
	})
	f.deps.cfg = &config.Config{DefaultBranch: "main"}

	if err := f.exec(newPruneCmd); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.wt.removedBranches) != 1 || f.wt.removedBranches[0] != "feature/merged" {
		t.Errorf("removed = %v, want only feature/merged", f.wt.removedBranches)
	}
}

// Without default_branch configured, prune compares against the branch checked
// out in the main worktree.
func TestPruneCmd_FallsBackToCurrentBranch(t *testing.T) {
	t.Parallel()

	f := newCmdFixture(&mockWorktreeManager{
		listResult:    cuttings("feature/a"),
		mergedResult:  []string{"develop", "feature/a"},
		currentBranch: "develop",
	})

	if err := f.exec(newPruneCmd); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.wt.mergedBase != "develop" {
		t.Errorf("merge base = %q, want the current branch %q", f.wt.mergedBase, "develop")
	}
}

func TestPruneCmd_CurrentBranchError(t *testing.T) {
	t.Parallel()

	f := newCmdFixture(&mockWorktreeManager{currentBranchErr: errors.New("detached")})
	if err := f.exec(newPruneCmd); err == nil {
		t.Fatal("expected the CurrentBranch error to propagate")
	}
}

func TestPruneCmd_DryRun(t *testing.T) {
	t.Parallel()

	f := newCmdFixture(&mockWorktreeManager{
		listResult:   cuttings("feature/a"),
		mergedResult: []string{"main", "feature/a"},
	})
	f.deps.cfg = &config.Config{DefaultBranch: "main"}

	if err := f.exec(newPruneCmd, "--dry-run"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.wt.removeCalled {
		t.Error("--dry-run must not remove anything")
	}
	if !strings.Contains(f.out(), `Would remove cutting for "feature/a" (merged into "main").`) {
		t.Errorf("output = %q, want the dry-run message naming the merge base", f.out())
	}
}

func TestPruneCmd_NothingToPrune(t *testing.T) {
	t.Parallel()

	f := newCmdFixture(&mockWorktreeManager{
		listResult:   cuttings("feature/a"),
		mergedResult: []string{"main"},
	})
	f.deps.cfg = &config.Config{DefaultBranch: "main"}

	if err := f.exec(newPruneCmd); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(f.out(), "No cuttings to prune.") {
		t.Errorf("output = %q, want the empty-case message", f.out())
	}
}

// --- init ---

func TestInitCmd_WritesConfigFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	f := newCmdFixture(&mockWorktreeManager{})
	f.deps.cfg = configForRepo(t, dir)

	if err := f.exec(newInitCmd); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, config.ConfigFileName)) //nolint:gosec // path is a test temp dir
	if err != nil {
		t.Fatalf("config file not written: %v", err)
	}
	if string(got) != string(config.DefaultFileContents()) {
		t.Error("written config does not match config.DefaultFileContents()")
	}
	if !strings.Contains(f.out(), "Created ") {
		t.Errorf("output = %q, want the created message", f.out())
	}
}

func TestInitCmd_RefusesToOverwrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, config.ConfigFileName)
	if err := os.WriteFile(path, []byte("# existing\n"), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	f := newCmdFixture(&mockWorktreeManager{})
	f.deps.cfg = configForRepo(t, dir)

	err := f.exec(newInitCmd)
	if err == nil {
		t.Fatal("expected an error when the config file already exists")
	}
	if !strings.Contains(err.Error(), "--overwrite") {
		t.Errorf("error = %q, want it to mention --overwrite", err)
	}

	got, _ := os.ReadFile(path) //nolint:gosec // path is a test temp dir
	if string(got) != "# existing\n" {
		t.Error("the existing config file was modified")
	}
}

func TestInitCmd_OverwriteReplacesExisting(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, config.ConfigFileName)
	if err := os.WriteFile(path, []byte("# existing\n"), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	f := newCmdFixture(&mockWorktreeManager{})
	f.deps.cfg = configForRepo(t, dir)

	if err := f.exec(newInitCmd, "--overwrite"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ := os.ReadFile(path) //nolint:gosec // path is a test temp dir
	if string(got) == "# existing\n" {
		t.Error("--overwrite did not replace the existing config file")
	}
}

// configForRepo returns a Config rooted at dir. Config.repoRoot is unexported,
// so it is obtained the same way the real command does: via config.Load.
func configForRepo(t *testing.T, dir string) *config.Config {
	t.Helper()
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return cfg
}

// --- completions ---

func TestCompleteCuttings_SkipsMainAndDetached(t *testing.T) {
	t.Parallel()

	trees := append(cuttings("feature/a"), worktree.Worktree{Path: "/repo/.worktrees/cut-run-1"})
	d := &Deps{wt: &mockWorktreeManager{listResult: trees}}

	got, directive := d.completeCuttings(nil, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v, want NoFileComp", directive)
	}
	if len(got) != 1 || !strings.HasPrefix(got[0], "feature/a\t") {
		t.Errorf("completions = %v, want only feature/a with its path", got)
	}
}

func TestCompleteCuttings_ListError(t *testing.T) {
	t.Parallel()

	d := &Deps{wt: &mockWorktreeManager{listErr: errors.New("boom")}}
	if _, directive := d.completeCuttings(nil, nil, ""); directive != cobra.ShellCompDirectiveError {
		t.Errorf("directive = %v, want Error", directive)
	}
}

// Completion runs before PersistentPreRunE has wired anything up when the
// command is invoked outside a repository, so a nil manager must not panic.
func TestCompletions_NilManager(t *testing.T) {
	t.Parallel()

	d := &Deps{}
	if got, _ := d.completeCuttings(nil, nil, ""); got != nil {
		t.Errorf("completeCuttings() = %v, want nil", got)
	}
	if got, _ := d.completeBranches(nil, nil, ""); got != nil {
		t.Errorf("completeBranches() = %v, want nil", got)
	}
}

func TestCompleteBranches(t *testing.T) {
	t.Parallel()

	d := &Deps{wt: &mockWorktreeManager{branches: []string{"main", "feature/a"}}}
	got, directive := d.completeBranches(nil, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v, want NoFileComp", directive)
	}
	if len(got) != 2 {
		t.Errorf("completions = %v, want both branches", got)
	}
}

// --- version ---

func TestVersionCmd(t *testing.T) {
	t.Parallel()

	cmd := newVersionCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "cuttings ") {
		t.Errorf("output = %q, want the version line", out.String())
	}
}
