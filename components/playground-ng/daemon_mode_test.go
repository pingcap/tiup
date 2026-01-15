package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	progressv2 "github.com/pingcap/tiup/pkg/tuiv2/progress"
	"github.com/stretchr/testify/require"
)

func TestBuildDaemonArgs_FiltersBackgroundAndTagFlags(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })

	os.Args = []string{
		"tiup-playground-ng",
		"--background",
		"--tag",
		"old",
		"--host",
		"127.0.0.1",
		"--run-as-daemon",
	}

	got := buildDaemonArgs("new")
	require.Equal(t, []string{
		"--host",
		"127.0.0.1",
		"--tag",
		"new",
		"--run-as-daemon",
	}, got)
}

func TestBuildDaemonArgs_FiltersShortTagForms(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })

	os.Args = []string{
		"tiup-playground-ng",
		"-d",
		"-Tfoo",
		"-T",
		"bar",
		"--tag=baz",
		"--host",
		"127.0.0.1",
	}

	got := buildDaemonArgs("final")
	require.Equal(t, []string{
		"--host",
		"127.0.0.1",
		"--tag",
		"final",
		"--run-as-daemon",
	}, got)
}

func TestBuildDaemonArgs_FiltersEqualsFormsAndKeepsPositionals(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })

	os.Args = []string{
		"tiup-playground-ng",
		"--background=true",
		"--run-as-daemon=true",
		"--tag=old",
		"v8.5.4",
		"--host",
		"127.0.0.1",
	}

	got := buildDaemonArgs("final")
	require.Equal(t, []string{
		"v8.5.4",
		"--host",
		"127.0.0.1",
		"--tag",
		"final",
		"--run-as-daemon",
	}, got)
}

func TestBackgroundStarterReadyMessage(t *testing.T) {
	require.Equal(t, "\n[dim]Cluster running in background (-d).[reset]\n[dim]To stop: [reset]tiup playground-ng stop --tag foo\n", backgroundStarterReadyMessage("foo"))
}

func TestTailEventLog_ReplaysNewEventsAfterOffset(t *testing.T) {
	dir := t.TempDir()
	eventLogPath := filepath.Join(dir, "events.jsonl")

	old, err := json.Marshal(progressv2.Event{
		Type:  progressv2.EventPrintLines,
		Lines: []string{"old"},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(eventLogPath, append(old, '\n'), 0o644))

	st, err := os.Stat(eventLogPath)
	require.NoError(t, err)
	offset := st.Size()

	outFile, err := os.CreateTemp(dir, "out")
	require.NoError(t, err)
	t.Cleanup(func() { _ = outFile.Close() })

	ui := progressv2.New(progressv2.Options{Mode: progressv2.ModePlain, Out: outFile})
	t.Cleanup(func() { _ = ui.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		tailEventLog(ctx, eventLogPath, offset, ui, nil)
		close(done)
	}()

	f, err := os.OpenFile(eventLogPath, os.O_WRONLY|os.O_APPEND, 0)
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })

	_, err = f.WriteString("not json\n")
	require.NoError(t, err)

	newEvent, err := json.Marshal(progressv2.Event{
		Type:  progressv2.EventPrintLines,
		Lines: []string{"new"},
	})
	require.NoError(t, err)

	half := len(newEvent) / 2
	_, err = f.Write(newEvent[:half])
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	_, err = f.Write(append(newEvent[half:], '\n'))
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		data, err := os.ReadFile(outFile.Name())
		if err != nil {
			return false
		}
		return strings.Contains(string(data), "new\n")
	}, time.Second, 20*time.Millisecond)

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting tailEventLog to stop")
	}

	data, err := os.ReadFile(outFile.Name())
	require.NoError(t, err)
	require.NotContains(t, string(data), "old")
	require.NotContains(t, string(data), "not json")
	require.Contains(t, string(data), "new\n")
}
