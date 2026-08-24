//go:build e2e

// Package e2e contains black-box tests that exercise the compiled
// workstreams binary as a subprocess against real, throwaway git
// repositories. It is gated behind the "e2e" build tag so that `go test
// ./...` (and the pre-commit go-test hook) skip it by default; run it
// explicitly with `make e2e`.
package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// binPath is the path to the workstreams binary built once in TestMain and
// reused by every test in this package.
var binPath string

// repoRoot is the root of the workstreams module, resolved independently of
// the test process's working directory.
var repoRoot string

func TestMain(m *testing.M) {
	os.Exit(runMain(m))
}

func runMain(m *testing.M) int {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		fmt.Fprintln(os.Stderr, "e2e: could not determine this file's location")
		return 1
	}
	// This file lives at <repoRoot>/e2e/main_test.go.
	repoRoot = filepath.Dir(filepath.Dir(thisFile))

	binDir, err := os.MkdirTemp("", "workstreams-e2e-bin-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: create temp dir: %v\n", err)
		return 1
	}
	defer func() { _ = os.RemoveAll(binDir) }()

	binPath = filepath.Join(binDir, "workstreams")
	ldflags := "-X github.com/ChristophBe/workstreams/cmd.Version=e2e-test " +
		"-X github.com/ChristophBe/workstreams/cmd.BuildTime=e2e-build-time"

	//nolint:gosec // repoRoot/binPath are derived from this test file's own location, not external input.
	build := exec.Command("go", "build", "-ldflags", ldflags, "-o", binPath, ".")
	build.Dir = repoRoot
	if out, buildErr := build.CombinedOutput(); buildErr != nil {
		fmt.Fprintf(os.Stderr, "e2e: build workstreams binary: %v\n%s\n", buildErr, out)
		return 1
	}

	return m.Run()
}
