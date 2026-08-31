/*
Copyright © 2026 Christoph Becker
*/

// Command gendocs regenerates the generated command reference embedded in
// docs/features.md from the Cobra command tree defined in package cmd.
//
// Run it via `make generate-docs`.
package main

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"github.com/ChristophBe/cuttings/cmd"
)

const (
	docsPath     = "docs/features.md"
	beginMarker  = "<!-- BEGIN GENERATED COMMANDS: run `make generate-docs` to update, do not edit by hand -->"
	endMarker    = "<!-- END GENERATED COMMANDS -->"
	seeAlsoTitle = "### SEE ALSO"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "gendocs:", err)
		os.Exit(1)
	}
}

func run() error {
	root := cmd.RootCmd()
	disableAutoGenTag(root)

	generated, err := generateCommandsSection(root)
	if err != nil {
		return err
	}

	original, err := os.ReadFile(docsPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", docsPath, err)
	}

	updated, err := spliceBetweenMarkers(string(original), generated)
	if err != nil {
		return fmt.Errorf("update %s: %w", docsPath, err)
	}

	if err := os.WriteFile(docsPath, []byte(updated), 0o644); err != nil { //nolint:gosec // matches existing doc file permissions
		return fmt.Errorf("write %s: %w", docsPath, err)
	}
	return nil
}

// disableAutoGenTag recursively disables cobra/doc's "Auto generated on
// <date>" footer. Without this the output would differ every day even with
// no real changes, which would defeat the CI drift check.
func disableAutoGenTag(c *cobra.Command) {
	c.DisableAutoGenTag = true
	for _, child := range c.Commands() {
		disableAutoGenTag(child)
	}
}

// linkHandler turns cobra/doc's default cross-reference filename (e.g.
// "cuttings_new.md") into an in-page anchor ("#cuttings-new") matching
// GitHub's heading slug for "## cuttings new", since every command is
// rendered into this single file rather than one file per command.
func linkHandler(name string) string {
	name = strings.TrimSuffix(name, ".md")
	return "#" + strings.ReplaceAll(name, "_", "-")
}

// generateCommandsSection renders one doc block per visible direct
// subcommand of root, in alphabetical order, nested one heading level below
// the file's own "## Commands" heading.
func generateCommandsSection(root *cobra.Command) (string, error) {
	var subs []*cobra.Command
	for _, c := range root.Commands() {
		if !c.IsAvailableCommand() {
			continue
		}
		subs = append(subs, c)
	}
	sort.Slice(subs, func(i, j int) bool { return subs[i].Name() < subs[j].Name() })

	var out strings.Builder
	for i, c := range subs {
		if i > 0 {
			out.WriteString("\n")
		}
		var buf bytes.Buffer
		if err := doc.GenMarkdownCustom(c, &buf, linkHandler); err != nil {
			return "", fmt.Errorf("generate docs for %q: %w", c.CommandPath(), err)
		}
		out.WriteString(nestHeadings(addAliases(stripSeeAlso(buf.String()), c.Aliases)))
	}
	return strings.TrimRight(out.String(), "\n") + "\n", nil
}

// stripSeeAlso removes cobra/doc's trailing "SEE ALSO" section (and the
// auto-gen footer that follows it), which only links back to the root
// command page — not useful once everything lives in one file.
func stripSeeAlso(s string) string {
	if idx := strings.Index(s, seeAlsoTitle); idx >= 0 {
		s = s[:idx]
	}
	return strings.TrimRight(s, "\n") + "\n"
}

// addAliases appends any command aliases (e.g. "rm" for "remove") to the
// generated top heading — cobra/doc's default template omits them, but the
// old hand-written docs called them out explicitly.
func addAliases(s string, aliases []string) string {
	if len(aliases) == 0 {
		return s
	}
	heading, rest, found := strings.Cut(s, "\n")
	if !found || !strings.HasPrefix(heading, "## ") {
		return s
	}
	label := "alias"
	if len(aliases) > 1 {
		label = "aliases"
	}
	return fmt.Sprintf("%s (%s: %s)\n%s", heading, label, strings.Join(aliases, ", "), rest)
}

// nestHeadings demotes every Markdown heading by one level so cobra/doc's
// "## <command>" top heading (and its "### ..." subsections) nest under this
// file's existing "## Commands" heading instead of competing with it.
func nestHeadings(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "#") {
			lines[i] = "#" + line
		}
	}
	return strings.Join(lines, "\n")
}

func spliceBetweenMarkers(original, generated string) (string, error) {
	begin := strings.Index(original, beginMarker)
	if begin < 0 {
		return "", fmt.Errorf("marker %q not found", beginMarker)
	}
	end := strings.Index(original, endMarker)
	if end < 0 {
		return "", fmt.Errorf("marker %q not found", endMarker)
	}
	if end < begin {
		return "", fmt.Errorf("end marker appears before begin marker")
	}

	var b strings.Builder
	b.WriteString(original[:begin])
	b.WriteString(beginMarker)
	b.WriteString("\n\n")
	b.WriteString(generated)
	b.WriteString("\n")
	b.WriteString(original[end:])
	return b.String(), nil
}
