package proc

import (
	"context"
	"fmt"
	"net"
	"time"

	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
)

// ReadyWaiter is an optional interface implemented by instances that need an
// explicit "ready" check after the process has been started.
//
// It is intentionally separated from the core Process interface so most
// components can remain "start-and-done", while components like TiDB / TiProxy /
// TiFlash can keep showing a spinner until they are actually ready to serve.
type ReadyWaiter interface {
	// WaitReady blocks until the instance is ready, or ctx is done.
	//
	// The maximum waiting time should usually be controlled by the instance's
	// UpTimeout field (in seconds). A value <= 0 means no limit.
	WaitReady(ctx context.Context) error
}

func tcpAddrReady(ctx context.Context, addr string, timeoutSec int) error {
	if addr == "" {
		return fmt.Errorf("empty address")
	}

	ctx, cancel := withTimeoutSeconds(ctx, timeoutSec)
	defer cancel()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		// Use a short per-attempt timeout so overall timeout semantics match
		// timeoutSec, and so Ctrl+C can interrupt quickly.
		perAttempt := time.Second
		if deadline, ok := ctx.Deadline(); ok {
			remain := time.Until(deadline)
			if remain <= 0 {
				return fmt.Errorf("timeout (%ds)", timeoutSec)
			}
			if remain < perAttempt {
				perAttempt = remain
			}
		}

		conn, err := net.DialTimeout("tcp", addr, perAttempt)
		if err == nil {
			_ = conn.Close()
			return nil
		}

		select {
		case <-ctx.Done():
			err := ctx.Err()
			if err == context.DeadlineExceeded && timeoutSec > 0 {
				return fmt.Errorf("timeout (%ds)", timeoutSec)
			}
			return err
		case <-ticker.C:
		}
	}
}

func withTimeoutSeconds(ctx context.Context, timeoutSec int) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if timeoutSec > 0 {
		return context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	}
	return ctx, func() {}
}

func withLogger(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Value(logprinter.ContextKeyLogger).(*logprinter.Logger); ok {
		return ctx
	}
	return context.WithValue(ctx, logprinter.ContextKeyLogger, logprinter.NewLogger(""))
}
