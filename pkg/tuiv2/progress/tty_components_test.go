package progress

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestTTYTaskMetaAlignmentForCanceledTasks(t *testing.T) {
	g := &groupState{title: "Start instances"}
	g.tasks = []*taskState{
		{title: "TiKV Worker", status: taskStatusDone, meta: "meta-long"},
		{title: "TiDB", status: taskStatusCanceled, meta: "meta-short"},
	}

	ctx := ttyRenderContext{
		styles:  newTTYStyles(io.Discard),
		width:   200,
		spinner: "⠦",
		now:     time.Now(),
	}
	lines := ttyGroupComponent{group: g}.Lines(ctx, 1_000_000)
	require.GreaterOrEqual(t, len(lines), 3, "expected at least 3 lines (header + 2 tasks)")

	lineLong := ansi.Strip(lines[1])
	lineShort := ansi.Strip(lines[2])

	idxLong := strings.Index(lineLong, "meta-long")
	idxShort := strings.Index(lineShort, "meta-short")
	require.NotEqual(t, -1, idxLong, "meta strings not found:\n%s\n%s", lineLong, lineShort)
	require.NotEqual(t, -1, idxShort, "meta strings not found:\n%s\n%s", lineLong, lineShort)

	posLong := lipgloss.Width(lineLong[:idxLong])
	posShort := lipgloss.Width(lineShort[:idxShort])
	require.Equal(t, posLong, posShort, "meta columns not aligned:\n%s\n%s", lineLong, lineShort)
}

func TestTTYTaskHideIfFast(t *testing.T) {
	now := time.Now()

	ctx := ttyRenderContext{
		styles:  newTTYStyles(io.Discard),
		width:   200,
		spinner: "⠦",
		now:     now,
	}

	t.Run("hide-success", func(t *testing.T) {
		g := &groupState{title: "Start instances"}
		g.tasks = []*taskState{
			{title: "PD", status: taskStatusDone},
			{title: "Grafana", status: taskStatusDone, hideIfFast: true, revealAfter: 2 * time.Second, startAt: now.Add(-3 * time.Second), endAt: now},
		}

		lines := ttyGroupComponent{group: g}.Lines(ctx, 1_000_000)
		got := ansi.Strip(strings.Join(lines, "\n"))
		require.NotContains(t, got, "Grafana")
		require.Contains(t, got, "PD")
	})

	t.Run("show-error", func(t *testing.T) {
		g := &groupState{title: "Start instances"}
		g.tasks = []*taskState{
			{title: "PD", status: taskStatusDone},
			{title: "Grafana", status: taskStatusError, hideIfFast: true, message: "boom"},
		}

		lines := ttyGroupComponent{group: g}.Lines(ctx, 1_000_000)
		got := ansi.Strip(strings.Join(lines, "\n"))
		require.Contains(t, got, "Grafana")
	})

	t.Run("reveal-running-only-when-slow", func(t *testing.T) {
		g := &groupState{title: "Shutdown"}
		g.tasks = []*taskState{
			{title: "Grafana", status: taskStatusRunning, hideIfFast: true, revealAfter: 10 * time.Second, startAt: now.Add(-2 * time.Second)},
		}

		lines := ttyGroupComponent{group: g}.Lines(ctx, 1_000_000)
		got := ansi.Strip(strings.Join(lines, "\n"))
		require.NotContains(t, got, "Grafana")

		g.tasks[0].revealAfter = 1 * time.Second
		lines = ttyGroupComponent{group: g}.Lines(ctx, 1_000_000)
		got = ansi.Strip(strings.Join(lines, "\n"))
		require.Contains(t, got, "Grafana")
	})
}

func TestTTYDownloadTask_ShowsRetryingMessage(t *testing.T) {
	g := &groupState{title: "Download components"}
	g.tasks = []*taskState{
		{title: "Grafana", kind: taskKindDownload, status: taskStatusDone, meta: "v8.5.4", total: 60 * 1024 * 1024},
		{title: "PD", kind: taskKindDownload, status: taskStatusDone, meta: "v8.5.4", total: 51 * 1024 * 1024},
		{title: "Prometheus", kind: taskKindDownload, status: taskStatusRetrying, meta: "v8.5.4", total: 126 * 1024 * 1024, message: "retrying 1/5..."},
		{title: "TiKV", kind: taskKindDownload, status: taskStatusPending, meta: "v8.5.4"},
	}

	ctx := ttyRenderContext{
		styles:  newTTYStyles(io.Discard),
		width:   200,
		spinner: "⠦",
		now:     time.Now(),
	}
	lines := ttyGroupComponent{group: g}.Lines(ctx, 1_000_000)
	got := ansi.Strip(strings.Join(lines, "\n"))
	require.Contains(t, got, "! Prometheus v8.5.4 (126MiB)  retrying 1/5...")
}
