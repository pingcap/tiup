package progress

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type blockingEventLogWriter struct {
	unblockOnce sync.Once
	unblockCh   chan struct{}
}

func (w *blockingEventLogWriter) Unblock() {
	if w == nil {
		return
	}
	w.unblockOnce.Do(func() {
		if w.unblockCh != nil {
			close(w.unblockCh)
		}
	})
}

func (w *blockingEventLogWriter) Write(p []byte) (int, error) {
	if w == nil {
		return len(p), nil
	}
	if w.unblockCh != nil {
		<-w.unblockCh
	}
	return len(p), nil
}

func TestUI_Sync_WaitsForEventLogWrites(t *testing.T) {
	outFile, err := os.CreateTemp(t.TempDir(), "ui-out")
	require.NoError(t, err)
	t.Cleanup(func() { _ = outFile.Close() })

	bw := &blockingEventLogWriter{unblockCh: make(chan struct{})}
	t.Cleanup(bw.Unblock)

	ui := New(Options{
		Mode:     ModePlain,
		Out:      outFile,
		EventLog: bw,
	})
	t.Cleanup(func() { _ = ui.Close() })

	_, _ = ui.Writer().Write([]byte("hello\n"))

	done := make(chan struct{})
	go func() {
		ui.Sync()
		close(done)
	}()

	select {
	case <-done:
		require.FailNow(t, "expected Sync to block while event log writer is blocked")
	case <-time.After(50 * time.Millisecond):
	}

	bw.Unblock()

	select {
	case <-done:
	case <-time.After(time.Second):
		require.FailNow(t, "timeout waiting for Sync to return")
	}
}
