package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/pingcap/tiup/pkg/repository"
	progressv2 "github.com/pingcap/tiup/pkg/tuiv2/progress"
	"github.com/stretchr/testify/require"
)

func TestRepoDownloadProgress_EventLog_RecordsTaskLifecycle(t *testing.T) {
	f, err := os.CreateTemp("", "tiup-playground-eventlog-*.log")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(f.Name()) })

	now := time.Unix(1_000_000, 0)
	nowFn := func() time.Time {
		now = now.Add(2 * time.Second)
		return now
	}

	var eventBuf bytes.Buffer

	ui := progressv2.New(progressv2.Options{
		Mode:     progressv2.ModePlain,
		Out:      f,
		EventLog: &eventBuf,
		Now:      nowFn,
	})

	g := ui.Group("Download components")
	ctx := context.Background()
	progress := newRepoDownloadProgress(ctx, g)
	reporter, ok := progress.(repository.DownloadProgressReporter)
	require.True(t, ok, "expected DownloadProgressReporter extension")

	rawURL := "https://example.com/tidb-v7.1.0-linux-amd64.tar.gz"
	progress.Start(rawURL, 123)
	progress.SetCurrent(50)
	reporter.Success(rawURL)
	g.Close()

	require.NoError(t, ui.Close())
	require.NoError(t, f.Close())

	data, err := io.ReadAll(&eventBuf)
	require.NoError(t, err)

	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	require.NotEmpty(t, lines, "expected event log to contain at least one line")

	var events []progressv2.Event
	for _, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		e, err := progressv2.DecodeEvent(line)
		require.NoError(t, err)
		events = append(events, e)
	}

	taskTitleByID := make(map[uint64]string)
	for _, e := range events {
		if e.Type == progressv2.EventTaskAdd && e.Title != nil {
			taskTitleByID[e.TaskID] = *e.Title
		}
	}

	tid := uint64(0)
	for id, title := range taskTitleByID {
		if title == "TiDB" {
			tid = id
			break
		}
	}
	require.NotZero(t, tid, "expected to find a TiDB task in event log")

	var (
		seenKindDownload bool
		seenMetaV710     bool
		seenTotal123     bool
		seenCurrent50    bool
		seenRunning      bool
		seenDone         bool
	)
	for _, e := range events {
		if e.TaskID != tid {
			continue
		}
		switch e.Type {
		case progressv2.EventTaskUpdate:
			if e.Kind != nil && *e.Kind == progressv2.TaskKindDownload {
				seenKindDownload = true
			}
			if e.Meta != nil && *e.Meta == "v7.1.0" {
				seenMetaV710 = true
			}
		case progressv2.EventTaskProgress:
			if e.Total != nil && *e.Total == 123 {
				seenTotal123 = true
			}
			if e.Current != nil && *e.Current == 50 {
				seenCurrent50 = true
			}
		case progressv2.EventTaskState:
			if e.Status == nil {
				continue
			}
			switch *e.Status {
			case progressv2.TaskStatusRunning:
				seenRunning = true
			case progressv2.TaskStatusDone:
				seenDone = true
			}
		}
	}

	require.True(t, seenKindDownload)
	require.True(t, seenMetaV710)
	require.True(t, seenTotal123)
	require.True(t, seenCurrent50)
	require.True(t, seenRunning)
	require.True(t, seenDone)
}
