package progress

import (
	"io"
	"strings"
	"testing"

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

