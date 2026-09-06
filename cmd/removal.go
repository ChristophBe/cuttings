/*
Copyright © 2026 Christoph Becker
*/

package cmd

import (
	"errors"
	"fmt"
	"io"

	"github.com/ChristophBe/cuttings/internal/worktree"
)

// cuttingBranches returns the branch names of every actual cutting in trees,
// skipping the main worktree and detached entries.
func cuttingBranches(trees []worktree.Worktree) []string {
	var branches []string
	for _, t := range worktree.Cuttings(trees) {
		branches = append(branches, t.Branch)
	}
	return branches
}

// removeEach removes the cutting for every branch in branches, reporting each
// success to out. Failures (e.g. uncommitted changes without --force) are
// collected rather than returned immediately, so one bad cutting does not
// block removal of the rest.
func removeEach(out io.Writer, wt cuttingRemover, branches []string, force bool) error {
	var errs []error
	for _, b := range branches {
		if err := wt.Remove(b, force); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", b, err))
			continue
		}
		_, _ = fmt.Fprintf(out, "Cutting for %q removed (branch preserved).\n", b)
	}
	return errors.Join(errs...)
}

// printWouldRemove reports a dry-run removal of branch. detail, when non-empty,
// is appended in parentheses (e.g. `merged into "main"`).
func printWouldRemove(out io.Writer, branch, detail string) {
	if detail == "" {
		_, _ = fmt.Fprintf(out, "Would remove cutting for %q.\n", branch)
		return
	}
	_, _ = fmt.Fprintf(out, "Would remove cutting for %q (%s).\n", branch, detail)
}
