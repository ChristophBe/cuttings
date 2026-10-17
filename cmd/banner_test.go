/*
Copyright © 2026 Christoph Becker
*/
package cmd

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestBannerIsPureASCII guards the constraint that matters most for a release
// that ships Windows binaries: the art must not rely on the terminal decoding
// UTF-8. It also rejects ANSI escape sequences, which would otherwise be
// invisible here but would corrupt the e2e tests' captured output.
func TestBannerIsPureASCII(t *testing.T) {
	t.Parallel()

	for i, r := range banner {
		switch {
		case r >= utf8.RuneSelf:
			t.Errorf("banner contains non-ASCII rune %q at byte %d", r, i)
		case r == 0x1b:
			t.Errorf("banner contains an ANSI escape at byte %d", i)
		case r == '\t':
			t.Errorf("banner contains a tab at byte %d; tab width is terminal-dependent", i)
		}
	}
}

// TestBannerHasNoBacktick keeps the art expressible as a raw string literal.
// A backtick would force it into an escaped, interpreted string, where the
// drawing is no longer readable in the source.
func TestBannerHasNoBacktick(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(banner, '`') {
		t.Error("banner contains a backtick, which cannot appear in a raw string literal")
	}
}

// TestBannerFitsNarrowTerminals checks every line against bannerMaxWidth, and
// checks that no line carries trailing whitespace — trailing spaces are
// invisible in the source but survive into the help output.
func TestBannerFitsNarrowTerminals(t *testing.T) {
	t.Parallel()

	for i, line := range strings.Split(banner, "\n") {
		if got := len(line); got > bannerMaxWidth {
			t.Errorf("banner line %d is %d columns, want <= %d: %q", i+1, got, bannerMaxWidth, line)
		}
		if line != strings.TrimRight(line, " ") {
			t.Errorf("banner line %d has trailing whitespace: %q", i+1, line)
		}
	}
}

// TestRootLongStartsWithBanner pins where the art is allowed to surface. Cobra
// renders Long for `cuttings --help` and for bare `cuttings`, which is the whole
// intended blast radius. The tree is built with an unresolved Deps: the banner
// is baked into the command's Long at construction time, so nothing here needs
// a repository.
func TestRootLongStartsWithBanner(t *testing.T) {
	t.Parallel()

	root := newRootCmd(&Deps{})

	if !strings.HasPrefix(root.Long, banner) {
		t.Errorf("root Long does not start with the banner; got prefix %q", truncate(root.Long, 40))
	}
	if !strings.Contains(root.Long, "isolated git working") {
		t.Error("root Long lost its descriptive text when the banner was prepended")
	}
}

// TestBannerStaysOffSubcommands is the real regression guard: the art belongs to
// root alone. Letting it reach a subcommand would also push it into
// docs/features.md, because tools/gendocs renders every subcommand's Long into
// the generated command reference — which the docs-check CI job then diffs.
func TestBannerStaysOffSubcommands(t *testing.T) {
	t.Parallel()

	for _, c := range newRootCmd(&Deps{}).Commands() {
		if strings.Contains(c.Long, banner) {
			t.Errorf("subcommand %q embeds the banner; it must stay on the root command only", c.Name())
		}
		if strings.Contains(c.Short, banner) {
			t.Errorf("subcommand %q embeds the banner in its Short description", c.Name())
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
