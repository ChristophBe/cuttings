//go:build e2e

package e2e

import (
	"strings"
	"testing"
)

// requireExitCode fails the test unless r.exitCode == want, printing stdout
// and stderr to aid debugging.
func requireExitCode(t *testing.T, r result, want int) {
	t.Helper()
	if r.exitCode != want {
		t.Fatalf("exit code = %d, want %d\nstdout:\n%s\nstderr:\n%s", r.exitCode, want, r.stdout, r.stderr)
	}
}

// requireContains fails the test unless s contains substr.
func requireContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Fatalf("expected %q to contain %q", s, substr)
	}
}

// requireNotContains fails the test if s contains substr.
func requireNotContains(t *testing.T, s, substr string) {
	t.Helper()
	if strings.Contains(s, substr) {
		t.Fatalf("expected %q not to contain %q", s, substr)
	}
}
