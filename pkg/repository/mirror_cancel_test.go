package repository

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestHTTPMirrorDownload_DoesNotStartWhenContextCanceled(t *testing.T) {
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	m := NewMirror(server.URL, MirrorOptions{Context: ctx, Progress: DisableProgress{}})
	if err := m.Open(); err != nil {
		t.Fatalf("open mirror: %v", err)
	}
	t.Cleanup(func() { _ = m.Close() })

	if err := m.Download("slow.tar.gz", t.TempDir()); err == nil {
		t.Fatalf("expected download to be canceled")
	}
	if got := atomic.LoadInt32(&requests); got != 0 {
		t.Fatalf("expected no request to be sent, got %d", got)
	}
}

func TestHTTPMirrorDownload_CanCancelByContext(t *testing.T) {
	var startedOnce sync.Once
	started := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/slow.tar.gz" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Length", "1024")
		if f, ok := w.(http.Flusher); ok {
			_, _ = w.Write(make([]byte, 1))
			f.Flush()
		} else {
			_, _ = w.Write(make([]byte, 1))
		}
		startedOnce.Do(func() { close(started) })
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m := NewMirror(server.URL, MirrorOptions{Context: ctx, Progress: DisableProgress{}})
	if err := m.Open(); err != nil {
		t.Fatalf("open mirror: %v", err)
	}
	t.Cleanup(func() { _ = m.Close() })

	targetDir := t.TempDir()

	errCh := make(chan error, 1)
	go func() {
		errCh <- m.Download("slow.tar.gz", targetDir)
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatalf("download did not start")
	}

	cancel()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatalf("expected download to be canceled")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("download did not stop after cancelation")
	}
}
