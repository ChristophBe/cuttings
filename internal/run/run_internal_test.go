/*
Copyright © 2026 Christoph Becker
*/

// Tests for the unexported signal helpers, which is why they live in package
// run rather than run_test. No real OS signals are involved: the channel is fed
// directly, exactly as signal.Notify would.
package run

import (
	"context"
	"errors"
	"os"
	"syscall"
	"testing"
)

func TestSignalAwareRun_NoSignal_ReturnsUnderlyingError(t *testing.T) {
	sigCh := make(chan os.Signal, 1)
	wantErr := errors.New("boom")

	sig, err := signalAwareRun(context.Background(), sigCh, func(_ context.Context) error {
		return wantErr
	})

	if !errors.Is(err, wantErr) {
		t.Errorf("err = %v, want %v", err, wantErr)
	}
	if sig != nil {
		t.Errorf("receivedSig = %v, want nil", sig)
	}
}

func TestSignalAwareRun_SignalDuringRun_CancelsContextAndReportsSignal(t *testing.T) {
	sigCh := make(chan os.Signal, 1)
	started := make(chan struct{})

	fn := func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}

	go func() {
		<-started
		sigCh <- syscall.SIGTERM
	}()

	sig, err := signalAwareRun(context.Background(), sigCh, fn)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if sig != syscall.SIGTERM {
		t.Errorf("receivedSig = %v, want SIGTERM", sig)
	}
}

func TestSignalExitCode(t *testing.T) {
	cases := []struct {
		sig  os.Signal
		want int
	}{
		{syscall.SIGINT, 130},
		{syscall.SIGTERM, 143},
		{syscall.SIGHUP, 129},
	}
	for _, c := range cases {
		if got := signalExitCode(c.sig); got != c.want {
			t.Errorf("signalExitCode(%v) = %d, want %d", c.sig, got, c.want)
		}
	}
}
