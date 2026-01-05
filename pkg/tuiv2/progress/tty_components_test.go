package progress

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestTTYTaskMetaAlignmentForCanceledTasks(t *testing.T) {
	g := &Group{title: "Starting instances"}
	g.tasks = []*Task{
		{title: "TiKV Worker", status: taskStatusDone, meta: "meta-long"},
		{title: "TiDB", status: taskStatusCanceled, meta: "meta-short"},
	}

	ctx := ttyRenderContext{
		styles:  newTTYStyles(io.Discard),
		width:   200,
		spinner: "⠦",
	}
	lines := ttyGroupComponent{group: g}.Lines(ctx, 1_000_000)
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines (header + 2 tasks), got %d", len(lines))
	}

	lineLong := ansi.Strip(lines[1])
	lineShort := ansi.Strip(lines[2])

	idxLong := strings.Index(lineLong, "meta-long")
	idxShort := strings.Index(lineShort, "meta-short")
	if idxLong < 0 || idxShort < 0 {
		t.Fatalf("meta strings not found:\n%s\n%s", lineLong, lineShort)
	}

	posLong := lipgloss.Width(lineLong[:idxLong])
	posShort := lipgloss.Width(lineShort[:idxShort])
	if posLong != posShort {
		t.Fatalf("meta columns not aligned: long=%d short=%d\n%s\n%s", posLong, posShort, lineLong, lineShort)
	}
}

func TestTTYTaskHideIfFast(t *testing.T) {
	now := time.Now()

	ctx := ttyRenderContext{
		styles:  newTTYStyles(io.Discard),
		width:   200,
		spinner: "⠦",
	}

	t.Run("hide-success", func(t *testing.T) {
		g := &Group{title: "Starting instances"}
		g.tasks = []*Task{
			{title: "PD", status: taskStatusDone},
			{title: "Grafana", status: taskStatusDone, hideIfFast: true, revealAfter: 2 * time.Second, startAt: now.Add(-3 * time.Second), endAt: now},
		}

		lines := ttyGroupComponent{group: g}.Lines(ctx, 1_000_000)
		got := ansi.Strip(strings.Join(lines, "\n"))
		if strings.Contains(got, "Grafana") {
			t.Fatalf("expected Grafana to be hidden, got:\n%s", got)
		}
		if !strings.Contains(got, "PD") {
			t.Fatalf("expected PD to be visible, got:\n%s", got)
		}
	})

	t.Run("show-error", func(t *testing.T) {
		g := &Group{title: "Starting instances"}
		g.tasks = []*Task{
			{title: "PD", status: taskStatusDone},
			{title: "Grafana", status: taskStatusError, hideIfFast: true, message: "boom"},
		}

		lines := ttyGroupComponent{group: g}.Lines(ctx, 1_000_000)
		got := ansi.Strip(strings.Join(lines, "\n"))
		if !strings.Contains(got, "Grafana") {
			t.Fatalf("expected Grafana error to be visible, got:\n%s", got)
		}
	})

	t.Run("reveal-running-only-when-slow", func(t *testing.T) {
		g := &Group{title: "Shutdown"}
		g.tasks = []*Task{
			{title: "Grafana", status: taskStatusRunning, hideIfFast: true, revealAfter: 10 * time.Second, startAt: now.Add(-2 * time.Second)},
		}

		lines := ttyGroupComponent{group: g}.Lines(ctx, 1_000_000)
		got := ansi.Strip(strings.Join(lines, "\n"))
		if strings.Contains(got, "Grafana") {
			t.Fatalf("expected Grafana to be hidden before revealAfter, got:\n%s", got)
		}

		g.tasks[0].revealAfter = 1 * time.Second
		lines = ttyGroupComponent{group: g}.Lines(ctx, 1_000_000)
		got = ansi.Strip(strings.Join(lines, "\n"))
		if !strings.Contains(got, "Grafana") {
			t.Fatalf("expected Grafana to be visible after revealAfter, got:\n%s", got)
		}
	})
}
