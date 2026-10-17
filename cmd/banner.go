/*
Copyright © 2026 Christoph Becker
*/

package cmd

// banner is the cuttings leaf mark rendered as ASCII art. It is prepended to
// the root command's Long description, so it appears above `cuttings --help`
// and above bare `cuttings` — and nowhere else. Subcommand help and the
// `version` command deliberately stay unadorned; `version` in particular keeps
// its single machine-readable line.
//
// The same leaf is the site's logo. Its vector sources live in branding/, and
// branding/README.md documents how the two stay in sync.
//
// Three constraints shape the art, all enforced by banner_test.go:
//
//   - Pure ASCII. Releases ship Windows binaries, and every other line this CLI
//     prints is plain ASCII too (cmd/list.go rules its header with "-"), so no
//     box-drawing characters.
//   - No ANSI colour. This codebase has no colour library, no TTY detection and
//     no NO_COLOR handling, and the e2e harness always captures stdout through a
//     pipe — unconditional escape sequences would land inside asserted output.
//   - No backtick, so the art can stay a readable raw string literal.
const banner = `        ,--.
      ,'   |
    ,'    /
   (    ,'
    '--'
   /
  /`

// bannerMaxWidth is the widest the art may get. Help output is routinely read in
// a split pane, so the mark is kept well inside the 80-column floor rather than
// merely under it.
const bannerMaxWidth = 20
