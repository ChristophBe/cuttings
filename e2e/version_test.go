//go:build e2e

package e2e

import "testing"

func TestVersion_OutsideRepo(t *testing.T) {
	dir := realPath(t, t.TempDir())
	h := newHarness(t, dir)

	r := h.run("version")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "workstreams e2e-test (built e2e-build-time)")
}

func TestVersion_InsideRepo(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)

	r := h.run("version")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, "workstreams e2e-test (built e2e-build-time)")
}
