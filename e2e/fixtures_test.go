//go:build e2e

package e2e

import "testing"

// newWorkstream runs `workstreams new <branch> [args...]` with the fake
// shell fixture and fails the test unless it succeeds. Used by tests that
// need an existing workstream as setup rather than exercising `new` itself.
func newWorkstream(t *testing.T, h *harness, branch string, extraArgs ...string) result {
	t.Helper()
	args := append([]string{"new", branch}, extraArgs...)
	r := h.withEnv("SHELL", fakeShellPath()).run(args...)
	requireExitCode(t, r, 0)
	return r
}
