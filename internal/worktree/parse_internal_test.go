/*
Copyright © 2026 Christoph Becker
*/

// These tests cover the unexported output parsers, which is why they live in
// package worktree rather than worktree_test.
package worktree

import "testing"

func TestParseBranchList(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want []string
	}{
		{name: "empty output", in: "", want: nil},
		{name: "only whitespace", in: "  \n\n  \n", want: nil},
		{name: "single branch", in: "main\n", want: []string{"main"}},
		{
			name: "several branches with a trailing newline",
			in:   "main\nfeature/a\nfeature/b\n",
			want: []string{"main", "feature/a", "feature/b"},
		},
		{
			name: "blank interior lines are dropped",
			in:   "main\n\nfeature/a\n",
			want: []string{"main", "feature/a"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := parseBranchList([]byte(c.in))
			if len(got) != len(c.want) {
				t.Fatalf("parseBranchList(%q) = %v, want %v", c.in, got, c.want)
			}
			for i := range c.want {
				if got[i] != c.want[i] {
					t.Errorf("parseBranchList(%q)[%d] = %q, want %q", c.in, i, got[i], c.want[i])
				}
			}
		})
	}
}
