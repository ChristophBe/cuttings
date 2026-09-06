/*
Copyright © 2026 Christoph Becker
*/

// These tests exercise the git command each method builds without running git
// or creating a repository, by replacing the Manager's runner. They live in
// package worktree because that seam is unexported. The behaviour of the real
// git invocations is covered by the integration tests in worktree_test.go.
package worktree

import (
	"errors"
	"strings"
	"testing"
)

// recordingManager returns a Manager rooted at a temp dir whose git calls are
// recorded instead of executed, plus a pointer to the recorded calls.
type gitCall struct {
	combined bool
	args     []string
}

func recordingManager(t *testing.T, out string, err error) (*Manager, *[]gitCall) {
	t.Helper()
	var calls []gitCall
	m := &Manager{
		repoRoot:     t.TempDir(),
		worktreesDir: ".worktrees",
		runner: func(_ string, combined bool, args ...string) ([]byte, error) {
			calls = append(calls, gitCall{combined: combined, args: args})
			return []byte(out), err
		},
	}
	return m, &calls
}

func joined(c gitCall) string { return strings.Join(c.args, " ") }

func TestAdd_BuildsGitArgs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		branch       string
		createBranch bool
		base         string
		wantSuffix   string
		wantPrefix   string
	}{
		{
			name:         "new branch without a base defaults to HEAD",
			branch:       "feature/foo",
			createBranch: true,
			wantPrefix:   "worktree add -b feature/foo ",
		},
		{
			name:         "new branch with a base appends the commit-ish",
			branch:       "feature/foo",
			createBranch: true,
			base:         "origin/main",
			wantPrefix:   "worktree add -b feature/foo ",
			wantSuffix:   " origin/main",
		},
		{
			name:       "existing branch is checked out, not created",
			branch:     "existing",
			wantPrefix: "worktree add ",
			wantSuffix: " existing",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			m, calls := recordingManager(t, "", nil)

			if _, err := m.Add(c.branch, c.createBranch, c.base); err != nil {
				t.Fatalf("Add() unexpected error: %v", err)
			}
			if len(*calls) != 1 {
				t.Fatalf("git called %d times, want 1: %v", len(*calls), *calls)
			}
			got := joined((*calls)[0])
			if c.wantPrefix != "" && !strings.HasPrefix(got, c.wantPrefix) {
				t.Errorf("git args = %q, want prefix %q", got, c.wantPrefix)
			}
			if c.wantSuffix != "" && !strings.HasSuffix(got, c.wantSuffix) {
				t.Errorf("git args = %q, want suffix %q", got, c.wantSuffix)
			}
			if !(*calls)[0].combined {
				t.Error("worktree add should use CombinedOutput so git's own message reaches the user")
			}
		})
	}
}

func TestAddDetached_BuildsGitArgs(t *testing.T) {
	t.Parallel()

	m, calls := recordingManager(t, "", nil)
	if _, err := m.AddDetached("cut-run-1", ""); err != nil {
		t.Fatalf("AddDetached() unexpected error: %v", err)
	}
	if got := joined((*calls)[0]); !strings.HasPrefix(got, "worktree add --detach ") {
		t.Errorf("git args = %q, want the --detach form", got)
	}

	m, calls = recordingManager(t, "", nil)
	if _, err := m.AddDetached("cut-run-1", "abc123"); err != nil {
		t.Fatalf("AddDetached() unexpected error: %v", err)
	}
	if got := joined((*calls)[0]); !strings.HasSuffix(got, " abc123") {
		t.Errorf("git args = %q, want the base appended", got)
	}
}

func TestRemove_ForceFlag(t *testing.T) {
	t.Parallel()

	// Remove stats the worktree directory first, so point it at one that
	// exists — the temp repo root itself.
	m, calls := recordingManager(t, "", nil)
	m.worktreesDir = "."

	if err := m.Remove(".", true); err != nil {
		t.Fatalf("Remove() unexpected error: %v", err)
	}
	if got := joined((*calls)[0]); !strings.Contains(got, "--force") {
		t.Errorf("git args = %q, want --force", got)
	}

	m, calls = recordingManager(t, "", nil)
	m.worktreesDir = "."
	if err := m.Remove(".", false); err != nil {
		t.Fatalf("Remove() unexpected error: %v", err)
	}
	if got := joined((*calls)[0]); strings.Contains(got, "--force") {
		t.Errorf("git args = %q, want no --force", got)
	}
}

func TestListMergedBranches_PassesBase(t *testing.T) {
	t.Parallel()

	m, calls := recordingManager(t, "main\nfeature/a\n", nil)

	got, err := m.ListMergedBranches("develop")
	if err != nil {
		t.Fatalf("ListMergedBranches() unexpected error: %v", err)
	}
	if want := "branch --format=%(refname:short) --merged develop"; joined((*calls)[0]) != want {
		t.Errorf("git args = %q, want %q", joined((*calls)[0]), want)
	}
	if len(got) != 2 {
		t.Errorf("ListMergedBranches() = %v, want both branches", got)
	}
}

// BranchExists checks local refs first and only falls back to the remote.
func TestBranchExists_ChecksLocalThenRemote(t *testing.T) {
	t.Parallel()

	m, calls := recordingManager(t, "", nil)
	if !m.BranchExists("feature/foo") {
		t.Error("BranchExists() = false, want true when the local ref resolves")
	}
	if len(*calls) != 1 {
		t.Fatalf("git called %d times, want 1 (no remote lookup needed)", len(*calls))
	}
	if want := "show-ref --verify --quiet refs/heads/feature/foo"; joined((*calls)[0]) != want {
		t.Errorf("git args = %q, want %q", joined((*calls)[0]), want)
	}

	m, calls = recordingManager(t, "", errors.New("no such ref"))
	if m.BranchExists("origin/feature/foo") {
		t.Error("BranchExists() = true, want false when neither ref resolves")
	}
	if len(*calls) != 2 {
		t.Fatalf("git called %d times, want 2 (local then remote)", len(*calls))
	}
	// The "origin/" prefix is stripped before building the remote ref, so the
	// lookup is not "refs/remotes/origin/origin/...".
	if want := "show-ref --verify --quiet refs/remotes/origin/feature/foo"; joined((*calls)[1]) != want {
		t.Errorf("remote lookup args = %q, want %q", joined((*calls)[1]), want)
	}
}

func TestGitErrorsAreWrapped(t *testing.T) {
	t.Parallel()

	boom := errors.New("git blew up")
	m, _ := recordingManager(t, "", boom)

	if _, err := m.List(); !errors.Is(err, boom) {
		t.Errorf("List() error = %v, want it to wrap %v", err, boom)
	}
	if _, err := m.ListBranches(); !errors.Is(err, boom) {
		t.Errorf("ListBranches() error = %v, want it to wrap %v", err, boom)
	}
	if _, err := m.CurrentBranch(); !errors.Is(err, boom) {
		t.Errorf("CurrentBranch() error = %v, want it to wrap %v", err, boom)
	}
	if _, err := m.GitCommonDir(); !errors.Is(err, boom) {
		t.Errorf("GitCommonDir() error = %v, want it to wrap %v", err, boom)
	}
}
