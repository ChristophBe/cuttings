//go:build e2e

package e2e

import "testing"

// leafTip is a slice of the ASCII leaf from cmd/banner.go, distinctive enough
// that it cannot collide with ordinary help prose. Tests match on this rather
// than on the whole drawing so that nudging the art does not break them — what
// is under test is *where* the banner appears, not its exact glyphs (that is
// cmd/banner_test.go's job).
const leafTip = `,--.`

// descriptiveText is a fragment of the root command's long description. Asserting
// it alongside the banner catches a prepend that replaced the description rather
// than preceding it.
const descriptiveText = "isolated git working"

func TestHelp_ShowsBannerInsideRepo(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)

	r := h.run("--help")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, leafTip)
	requireContains(t, r.stdout, descriptiveText)
}

// TestHelp_ShowsBannerOutsideRepo pins behaviour that is easy to break without
// noticing: the root command's PersistentPreRunE resolves a git repository root
// and fails outside one, but Cobra evaluates the help flag before running it, so
// `--help` must still work from an unrelated directory.
func TestHelp_ShowsBannerOutsideRepo(t *testing.T) {
	dir := realPath(t, t.TempDir())
	h := newHarness(t, dir)

	r := h.run("--help")
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, leafTip)
	requireContains(t, r.stdout, descriptiveText)
}

// TestHelp_BareInvocationShowsBanner covers `cuttings` with no arguments. The
// root command has no Run, so Cobra falls through to help — again before
// PersistentPreRunE, hence the deliberately repo-less working directory.
func TestHelp_BareInvocationShowsBanner(t *testing.T) {
	dir := realPath(t, t.TempDir())
	h := newHarness(t, dir)

	r := h.run()
	requireExitCode(t, r, 0)
	requireContains(t, r.stdout, leafTip)
	requireContains(t, r.stdout, descriptiveText)
}

// TestHelp_SubcommandsHaveNoBanner keeps the art scoped to root. Beyond being
// noisy, a banner on a subcommand's Long would be rendered into
// docs/features.md by tools/gendocs and fail the docs-check drift gate.
func TestHelp_SubcommandsHaveNoBanner(t *testing.T) {
	dir := initRepo(t)
	h := newHarness(t, dir)

	for _, sub := range []string{"list", "new", "remove", "run", "shell", "prune", "init", "version"} {
		t.Run(sub, func(t *testing.T) {
			r := h.run(sub, "--help")
			requireExitCode(t, r, 0)
			requireNotContains(t, r.stdout, leafTip)
		})
	}
}
