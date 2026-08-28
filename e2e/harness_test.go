//go:build e2e

package e2e

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
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

// buildEnv returns the explicit subprocess environment for this harness.
func (h *harness) buildEnv() []string {
	env := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + h.home}
	for k, v := range h.env {
		env = append(env, k+"="+v)
	}
	return env
}

// run invokes the workstreams binary with args and blocks until it exits,
// returning its result. It never fails the test on a non-zero exit code —
// callers assert that explicitly, since a non-zero exit is often the
// expected outcome. Use start instead when the test needs to interact with
// the process (e.g. send it a signal) before it exits.
//
// Stdin is left unconnected (equivalent to /dev/null), so any read from it
// hits an immediate EOF — this is what exercises the "no terminal attached"
// default for interactive prompts. Use runWithStdin to script an answer.
func (h *harness) run(args ...string) result {
	h.t.Helper()
	return h.runWithStdin("", args...)
}

// runWithStdin behaves like run, but connects stdin to a reader over the
// given string — for scripting an answer to an interactive prompt (e.g.
// "y\n" or "n\n" for the `run --branch <existing>` removal confirmation).
func (h *harness) runWithStdin(stdin string, args ...string) result {
	h.t.Helper()

	//nolint:gosec // binPath is our own freshly-built test binary, not user input.
	cmd := exec.Command(binPath, args...)
	cmd.Dir = h.dir
	cmd.Env = h.buildEnv()
	cmd.Stdin = strings.NewReader(stdin)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	exitCode := exitCodeFromErr(h.t, runErr)

	return result{stdout: stdout.String(), stderr: stderr.String(), exitCode: exitCode}
}

// asyncRun is a workstreams invocation started in the background via
// harness.start, so the test can interact with it (typically: send a
// signal) before collecting its result with wait.
type asyncRun struct {
	t      *testing.T
	cmd    *exec.Cmd
	stdout *bytes.Buffer
	stderr *bytes.Buffer
}

// start begins running the workstreams binary with args in the background
// and returns immediately, without waiting for it to exit.
func (h *harness) start(args ...string) *asyncRun {
	h.t.Helper()

	//nolint:gosec // binPath is our own freshly-built test binary, not user input.
	cmd := exec.Command(binPath, args...)
	cmd.Dir = h.dir
	cmd.Env = h.buildEnv()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		h.t.Fatalf("start %v: %v", args, err)
	}

	return &asyncRun{t: h.t, cmd: cmd, stdout: &stdout, stderr: &stderr}
}

// signal sends sig to the running process.
func (r *asyncRun) signal(sig os.Signal) {
	r.t.Helper()
	if err := r.cmd.Process.Signal(sig); err != nil {
		r.t.Fatalf("signal %v: %v", sig, err)
	}
}

// wait blocks until the process exits and returns its result. Only safe to
// call once, and only after the process has actually started (i.e. after
// start returned) — stdout/stderr are read here, once exec.Cmd guarantees
// the output-copying goroutines it started internally have finished.
func (r *asyncRun) wait() result {
	r.t.Helper()
	runErr := r.cmd.Wait()
	exitCode := exitCodeFromErr(r.t, runErr)
	return result{stdout: r.stdout.String(), stderr: r.stderr.String(), exitCode: exitCode}
}

// exitCodeFromErr extracts a process exit code from the error returned by
// exec.Cmd's Run/Wait, failing the test if err is a non-exit error (e.g. the
// binary itself could not be started). A process terminated by a signal
// (rather than exiting normally) reports -1, per exec.ExitError.ExitCode.
func exitCodeFromErr(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("run: %v", err)
	}
	return exitErr.ExitCode()
}
