//go:build e2e

package e2e

import "testing"

func TestCompletion_Shells(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)

	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		t.Run(shell, func(t *testing.T) {
			r := h.run("completion", shell)
			requireExitCode(t, r, 0)
			if r.stdout == "" {
				t.Fatalf("expected non-empty completion script for %s", shell)
			}
			requireContains(t, r.stdout, "cuttings")
		})
	}
}

func TestCompletion_DynamicCuttings(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)
	newCutting(t, h, "feature/foo")

	r := h.run("__complete", "shell", "")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "feature/foo")
}
