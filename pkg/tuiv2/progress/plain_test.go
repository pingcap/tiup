package progress

import (
	"io"
	"os"
	"testing"
	"time"

	"github.com/pingcap/tiup/pkg/tui/colorstr"
	"github.com/stretchr/testify/require"
)

func TestPlainOutput_IsStableAndNoANSI(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })
	t.Cleanup(func() { _ = w.Close() })

	ui := New(Options{Mode: ModePlain, Out: w})

	g := ui.Group("Waiting for things")
	t1 := g.Task("task-ok")
	t1.Start()
	t1.Done()
	t2 := g.Task("task-err")
	t2.Start()
	t2.Error("boom")
	g.Close()

	require.NoError(t, ui.Close())
	_ = w.Close()
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	got := string(out)

	require.NotContains(t, got, "\033[", "plain output must not contain ANSI sequences")
	for _, want := range []string{
		"Waiting for things | task-ok\n",
		"Waiting for things | task-err\n",
		"Waiting for things | ERR - task-err: boom (",
	} {
		require.Contains(t, got, want)
	}
}

func TestDownloadTask_Plain(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })
	t.Cleanup(func() { _ = w.Close() })

	ui := New(Options{Mode: ModePlain, Out: w})

	g := ui.Group("Download components")
	t1 := g.Task("TiDB")
	t1.SetTotal(1024 * 1024)
	t1.SetMeta("v7.5.0")
	t1.SetKindDownload()
	t1.SetCurrent(512 * 1024)
	t1.SetCurrent(1024 * 1024)
	t1.Done()
	g.Close()

	require.NoError(t, ui.Close())
	_ = w.Close()
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	got := string(out)

	require.NotContains(t, got, "\033[", "plain output must not contain ANSI sequences")
	for _, want := range []string{
		"Download components | TiDB v7.5.0 (1.0MiB)\n",
	} {
		require.Contains(t, got, want)
	}
}

func TestDownloadTask_Plain_PendingTaskTotalBeforeStart(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })
	t.Cleanup(func() { _ = w.Close() })

	ui := New(Options{Mode: ModePlain, Out: w})

	g := ui.Group("Download components")
	t1 := g.TaskPending("TiDB")
	t1.SetTotal(1024 * 1024)
	t1.SetMeta("v7.5.0")
	t1.SetKindDownload()
	t1.Start()
	t1.SetCurrent(1024 * 1024)
	t1.Done()
	g.Close()

	require.NoError(t, ui.Close())
	_ = w.Close()
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	got := string(out)

	for _, want := range []string{
		"Download components | TiDB v7.5.0 (1.0MiB)\n",
	} {
		require.Contains(t, got, want)
	}
}

func TestPlainOutput_FORCE_COLOR_EmitsANSI(t *testing.T) {
	t.Setenv("FORCE_COLOR", "1")

	r, w, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })
	t.Cleanup(func() { _ = w.Close() })

	ui := New(Options{Mode: ModePlain, Out: w})

	g := ui.Group("Waiting for things")
	t1 := g.Task("task-err")
	t1.Start()
	t1.Error("boom")
	g.Close()

	require.NoError(t, ui.Close())
	_ = w.Close()
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	got := string(out)

	require.Contains(t, got, "\033[", "expected ANSI sequences with FORCE_COLOR")

	tokens := colorstr.DefaultTokens
	tokens.Disable = false

	require.Contains(t, got, tokens.Sprintf("[light_magenta][bold]Waiting for things[reset]"), "expected colored group prefix")
	require.Contains(t, got, tokens.Sprintf("[bold][light_red]ERR[reset]"), "expected colored ERR label")
}

func TestGroupElapsed_FreezeWhenAllTasksDone(t *testing.T) {
	start := time.Unix(1_000_000, 0)
	end := start.Add(10 * time.Second)

	g := &groupState{startedAt: start}
	g.tasks = []*taskState{
		{status: taskStatusDone, startAt: start, endAt: end},
	}

	require.Equal(t, 10*time.Second, g.elapsed(end))
}

func TestGroupElapsed_DoesNotFreezeUntilTasksDone(t *testing.T) {
	start := time.Unix(1_000_000, 0)
	closeAt := start.Add(2 * time.Second)
	end := start.Add(10 * time.Second)

	g := &groupState{
		startedAt: start,
		closed:    true,
		closedAt:  closeAt,
	}
	g.tasks = []*taskState{
		{status: taskStatusDone, startAt: start, endAt: end},
	}

	require.Equal(t, 10*time.Second, g.elapsed(end))
}

func TestPendingTask_Cancel_PrintsInPlain(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })
	t.Cleanup(func() { _ = w.Close() })

	now := time.Unix(1_000_000, 0)
	ui := New(Options{
		Mode: ModePlain,
		Out:  w,
		Now:  func() time.Time { return now },
	})

	g := ui.Group("Start instances")
	t1 := g.TaskPending("TiDB")
	t1.Cancel("")
	g.Close()

	require.NoError(t, ui.Close())
	_ = w.Close()
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	got := string(out)

	require.Contains(t, got, "Start instances | CANCEL - TiDB (0.0s)\n")
}
