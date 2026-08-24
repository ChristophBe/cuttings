//go:build e2e

package e2e

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"testing"
)

// result captures the outcome of one workstreams invocation.
type result struct {
	stdout   string
	stderr   string
	exitCode int
}

// harness invokes the compiled workstreams binary as a subprocess with a
// fixed working directory and a minimal, explicit environment, so tests are
// hermetic regardless of the developer's or CI runner's real environment.
type harness struct {
	t    *testing.T
	dir  string
	home string
	env  map[string]string
}

// newHarness returns a harness rooted at dir, with its own isolated $HOME so
// no real ~/.gitconfig or ~/.workstreams* leaks into the test.
func newHarness(t *testing.T, dir string) *harness {
	t.Helper()
	return &harness{
		t:    t,
		dir:  dir,
		home: t.TempDir(),
		env:  map[string]string{},
	}
}

// withEnv returns a copy of h with key=value added to the subprocess
// environment (e.g. SHELL, or a WORKSTREAMS_* override).
func (h *harness) withEnv(key, value string) *harness {
	env := make(map[string]string, len(h.env)+1)
	for k, v := range h.env {
		env[k] = v
	}
	env[key] = value
	return &harness{t: h.t, dir: h.dir, home: h.home, env: env}
}

// run invokes the workstreams binary with args and returns its result. It
// never fails the test on a non-zero exit code — callers assert that
// explicitly, since a non-zero exit is often the expected outcome.
func (h *harness) run(args ...string) result {
	h.t.Helper()

	//nolint:gosec // binPath is our own freshly-built test binary, not user input.
	cmd := exec.Command(binPath, args...)
	cmd.Dir = h.dir

	env := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + h.home}
	for k, v := range h.env {
		env = append(env, k+"="+v)
	}
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(runErr, &exitErr) {
			h.t.Fatalf("run %v: %v", args, runErr)
		}
		exitCode = exitErr.ExitCode()
	}

	return result{stdout: stdout.String(), stderr: stderr.String(), exitCode: exitCode}
}
