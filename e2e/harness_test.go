//go:build e2e

package e2e

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// result captures the outcome of one cuttings invocation.
type result struct {
	stdout   string
	stderr   string
	exitCode int
}

// harness invokes the compiled cuttings binary as a subprocess with a
// fixed working directory and a minimal, explicit environment, so tests are
// hermetic regardless of the developer's or CI runner's real environment.
type harness struct {
	t    *testing.T
	dir  string
	home string
	env  map[string]string
}

// newHarness returns a harness rooted at dir, with its own isolated $HOME so
// no real ~/.gitconfig or ~/.cuttings* leaks into the test.
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
// environment (e.g. SHELL, or a CUTTINGS_* override).
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

// run invokes the cuttings binary with args and blocks until it exits,
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

// asyncRun is a cuttings invocation started in the background via
// harness.start, so the test can interact with it (typically: send a
// signal) before collecting its result with wait.
type asyncRun struct {
	t   *testing.T
	cmd *exec.Cmd
	// stdoutPath and stderrPath are files the child writes to directly. See
	// start for why they are files rather than in-memory buffers.
	stdoutPath string
	stderrPath string
}

// outputFile creates a file for a background process to write to. It must be a
// real *os.File — see start.
func outputFile(t *testing.T, dir, name string) *os.File {
	t.Helper()
	f, err := os.Create(filepath.Join(dir, name)) //nolint:gosec // dir is a test temp dir.
	if err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

// start begins running the cuttings binary with args in the background
// and returns immediately, without waiting for it to exit.
//
// The child's output goes to files rather than to bytes.Buffers, and that
// choice is load-bearing. os/exec only hands a descriptor straight to the child
// when the writer is an *os.File; for anything else it inserts a pipe and
// copying goroutines that Wait blocks on. A command run inside a cutting can
// leave grandchildren holding the write end of that pipe long after cuttings
// itself has exited (a shell that spawned a background sleep, say), and wait()
// would then block on those strangers instead of on the process under test.
// Writing to files keeps wait() a question strictly about cuttings.
func (h *harness) start(args ...string) *asyncRun {
	h.t.Helper()

	//nolint:gosec // binPath is our own freshly-built test binary, not user input.
	cmd := exec.Command(binPath, args...)
	cmd.Dir = h.dir
	cmd.Env = h.buildEnv()

	outDir := h.t.TempDir()
	stdout := outputFile(h.t, outDir, "stdout")
	stderr := outputFile(h.t, outDir, "stderr")
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		h.t.Fatalf("start %v: %v", args, err)
	}

	return &asyncRun{t: h.t, cmd: cmd, stdoutPath: stdout.Name(), stderrPath: stderr.Name()}
}

// signal sends sig to the running process.
func (r *asyncRun) signal(sig os.Signal) {
	r.t.Helper()
	if err := r.cmd.Process.Signal(sig); err != nil {
		r.t.Fatalf("signal %v: %v", sig, err)
	}
}

// wait blocks until the process exits and returns its result. Only safe to
// call once, and only after the process has actually started (i.e. after start
// returned). It returns as soon as cuttings itself is gone — see start for why
// that is not true of the obvious in-memory implementation.
func (r *asyncRun) wait() result {
	r.t.Helper()
	runErr := r.cmd.Wait()
	exitCode := exitCodeFromErr(r.t, runErr)
	return result{
		stdout:   readFile(r.t, r.stdoutPath),
		stderr:   readFile(r.t, r.stderrPath),
		exitCode: exitCode,
	}
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
