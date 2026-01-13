package main

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestProcessGroupWait_BlocksUntilClose(t *testing.T) {
	g := NewProcessGroup()

	doneCh := make(chan error, 1)
	go func() { doneCh <- g.Wait() }()

	select {
	case err := <-doneCh:
		require.FailNow(t, "Wait returned early", "err=%v", err)
	case <-time.After(100 * time.Millisecond):
	}

	g.Close()

	select {
	case err := <-doneCh:
		require.NoError(t, err)
	case <-time.After(time.Second):
		require.FailNow(t, "Wait did not return after Close")
	}
}

func TestProcessGroupWait_WaitsForTasks(t *testing.T) {
	g := NewProcessGroup()

	blockCh := make(chan struct{})
	err := g.Add("block", func() error {
		<-blockCh
		return nil
	})
	require.NoError(t, err)

	doneCh := make(chan error, 1)
	go func() { doneCh <- g.Wait() }()

	g.Close()

	select {
	case <-doneCh:
		require.FailNow(t, "Wait returned before task completed")
	case <-time.After(100 * time.Millisecond):
	}

	close(blockCh)

	select {
	case err := <-doneCh:
		require.NoError(t, err)
	case <-time.After(time.Second):
		require.FailNow(t, "Wait did not return after task completed")
	}
}

func TestProcessGroupAdd_AfterCloseRejected(t *testing.T) {
	g := NewProcessGroup()
	g.Close()
	require.ErrorIs(t, g.Add("x", func() error { return nil }), errProcessGroupClosed)
}

func TestProcessGroupAdd_NilWaitFuncRejected(t *testing.T) {
	g := NewProcessGroup()
	require.Error(t, g.Add("x", nil))
}

func TestProcessGroupWait_ReturnsFirstError(t *testing.T) {
	g := NewProcessGroup()

	blockCh := make(chan struct{})
	require.NoError(t, g.Add("first", func() error { return errors.New("boom") }))
	require.NoError(t, g.Add("second", func() error {
		<-blockCh
		return errors.New("late")
	}))

	deadline := time.Now().Add(time.Second)
	for {
		g.mu.Lock()
		seen := g.firstErr
		g.mu.Unlock()
		if seen != nil {
			break
		}
		if time.Now().After(deadline) {
			require.FailNow(t, "timeout waiting for firstErr to be set")
		}
		time.Sleep(10 * time.Millisecond)
	}

	doneCh := make(chan error, 1)
	go func() { doneCh <- g.Wait() }()

	g.Close()
	close(blockCh)

	select {
	case err := <-doneCh:
		require.Error(t, err)
		require.Contains(t, err.Error(), "first:")
		require.Contains(t, err.Error(), "boom")
	case <-time.After(time.Second):
		require.FailNow(t, "Wait did not return")
	}
}
