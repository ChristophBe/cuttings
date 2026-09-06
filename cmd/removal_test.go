/*
Copyright © 2026 Christoph Becker
*/
package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/ChristophBe/cuttings/internal/worktree"
)

// stubRemover records the branches it was asked to remove and fails for any
// branch listed in failFor.
type stubRemover struct {
	failFor map[string]error
	removed []string
	forces  []bool
}

func (s *stubRemover) Remove(branch string, force bool) error {
	s.removed = append(s.removed, branch)
	s.forces = append(s.forces, force)
	if err, ok := s.failFor[branch]; ok {
		return err
	}
	return nil
}

func TestCuttingBranches(t *testing.T) {
	t.Parallel()

	trees := []worktree.Worktree{
		{Branch: "main", Path: "/repo", IsMain: true},
		{Branch: "feature/a", Path: "/repo/.worktrees/feature/a"},
		{Branch: "", Path: "/repo/.worktrees/cut-run-123"}, // detached run worktree
		{Branch: "feature/b", Path: "/repo/.worktrees/feature/b"},
	}

	got := cuttingBranches(trees)
	want := []string{"feature/a", "feature/b"}
	if len(got) != len(want) {
		t.Fatalf("cuttingBranches() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("cuttingBranches()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestCuttingBranches_NoCuttings(t *testing.T) {
	t.Parallel()

	trees := []worktree.Worktree{{Branch: "main", Path: "/repo", IsMain: true}}
	if got := cuttingBranches(trees); got != nil {
		t.Errorf("cuttingBranches() = %v, want nil", got)
	}
}

func TestRemoveEach_AllSucceed(t *testing.T) {
	t.Parallel()

	rm := &stubRemover{}
	var out bytes.Buffer

	if err := removeEach(&out, rm, []string{"feature/a", "feature/b"}, false); err != nil {
		t.Fatalf("removeEach() error = %v, want nil", err)
	}

	if len(rm.removed) != 2 {
		t.Fatalf("removed %v, want 2 branches", rm.removed)
	}
	for _, want := range []string{
		`Cutting for "feature/a" removed (branch preserved).`,
		`Cutting for "feature/b" removed (branch preserved).`,
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing %q, got:\n%s", want, out.String())
		}
	}
}

func TestRemoveEach_ForcePassedThrough(t *testing.T) {
	t.Parallel()

	rm := &stubRemover{}
	if err := removeEach(&bytes.Buffer{}, rm, []string{"feature/a"}, true); err != nil {
		t.Fatalf("removeEach() error = %v", err)
	}
	if len(rm.forces) != 1 || !rm.forces[0] {
		t.Errorf("force = %v, want [true]", rm.forces)
	}
}

// One failing cutting must not stop the others: this is why removeEach
// collects errors instead of returning on the first one.
func TestRemoveEach_PartialFailure_ContinuesAndJoins(t *testing.T) {
	t.Parallel()

	boom := errors.New("uncommitted changes")
	rm := &stubRemover{failFor: map[string]error{"feature/b": boom}}
	var out bytes.Buffer

	err := removeEach(&out, rm, []string{"feature/a", "feature/b", "feature/c"}, false)
	if err == nil {
		t.Fatal("removeEach() error = nil, want an error")
	}
	if !errors.Is(err, boom) {
		t.Errorf("removeEach() error = %v, want it to wrap %v", err, boom)
	}
	if !strings.Contains(err.Error(), "feature/b:") {
		t.Errorf("error %q should name the failing branch", err)
	}

	// All three were attempted, and the two that worked were reported.
	if len(rm.removed) != 3 {
		t.Errorf("removed %v, want all 3 attempted", rm.removed)
	}
	if strings.Contains(out.String(), "feature/b") {
		t.Errorf("failed branch should not be reported as removed, got:\n%s", out.String())
	}
	for _, want := range []string{"feature/a", "feature/c"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing successful removal of %q, got:\n%s", want, out.String())
		}
	}
}

func TestPrintWouldRemove(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		branch string
		detail string
		want   string
	}{
		{
			name:   "without detail",
			branch: "feature/a",
			want:   "Would remove cutting for \"feature/a\".\n",
		},
		{
			name:   "with detail",
			branch: "feature/a",
			detail: `merged into "main"`,
			want:   "Would remove cutting for \"feature/a\" (merged into \"main\").\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			var out bytes.Buffer
			printWouldRemove(&out, c.branch, c.detail)
			if got := out.String(); got != c.want {
				t.Errorf("printWouldRemove() = %q, want %q", got, c.want)
			}
		})
	}
}
