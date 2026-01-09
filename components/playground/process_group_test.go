package main

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestProcessGroupWait_BlocksUntilClose(t *testing.T) {
	g := NewProcessGroup()

	doneCh := make(chan error, 1)
	go func() { doneCh <- g.Wait() }()

	select {
	case err := <-doneCh:
		t.Fatalf("Wait returned early: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	g.Close()

	select {
	case err := <-doneCh:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("Wait did not return after Close")
	}
}

func TestProcessGroupWait_WaitsForTasks(t *testing.T) {
	g := NewProcessGroup()

	blockCh := make(chan struct{})
	if err := g.Add("block", func() error {
		<-blockCh
		return nil
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	doneCh := make(chan error, 1)
	go func() { doneCh <- g.Wait() }()

	g.Close()

	select {
	case <-doneCh:
		t.Fatalf("Wait returned before task completed")
	case <-time.After(100 * time.Millisecond):
	}

	close(blockCh)

	select {
	case err := <-doneCh:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("Wait did not return after task completed")
	}
}

func TestProcessGroupAdd_AfterCloseRejected(t *testing.T) {
	g := NewProcessGroup()
	g.Close()
	if err := g.Add("x", func() error { return nil }); !errors.Is(err, errProcessGroupClosed) {
		t.Fatalf("expected errProcessGroupClosed, got %v", err)
	}
}

func TestProcessGroupAdd_NilWaitFuncRejected(t *testing.T) {
	g := NewProcessGroup()
	if err := g.Add("x", nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestProcessGroupWait_ReturnsFirstError(t *testing.T) {
	g := NewProcessGroup()

	blockCh := make(chan struct{})
	if err := g.Add("first", func() error { return errors.New("boom") }); err != nil {
		t.Fatalf("Add first: %v", err)
	}
	if err := g.Add("second", func() error {
		<-blockCh
		return errors.New("late")
	}); err != nil {
		t.Fatalf("Add second: %v", err)
	}

	deadline := time.Now().Add(time.Second)
	for {
		g.mu.Lock()
		seen := g.firstErr
		g.mu.Unlock()
		if seen != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for firstErr to be set")
		}
		time.Sleep(10 * time.Millisecond)
	}

	doneCh := make(chan error, 1)
	go func() { doneCh <- g.Wait() }()

	g.Close()
	close(blockCh)

	select {
	case err := <-doneCh:
		if err == nil {
			t.Fatalf("expected error")
		}
		if !strings.Contains(err.Error(), "first:") || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("Wait did not return")
	}
}
