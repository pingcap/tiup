package progress

import (
	"fmt"
	"io"
	"time"

	"github.com/pingcap/tiup/pkg/tui/colorstr"
	tuiterm "github.com/pingcap/tiup/pkg/tui/term"
)

type plainRenderer struct {
	out     io.Writer
	outMode tuiterm.OutputMode
}

func newPlainRenderer(out io.Writer, outMode tuiterm.OutputMode) *plainRenderer {
	if out == nil {
		out = io.Discard
	}
	return &plainRenderer{out: out, outMode: outMode}
}

func (r *plainRenderer) plainSprintf(format string, args ...any) string {
	tokens := colorstr.DefaultTokens
	tokens.Disable = !r.outMode.Color
	return tokens.Sprintf(format, args...)
}

func (r *plainRenderer) groupPrefix(title string) string {
	if title == "" {
		return ""
	}
	return r.plainSprintf("[light_magenta][bold]%s[reset]", title)
}

func (r *plainRenderer) printlnWithGroup(g *groupState, details string) {
	if r == nil || r.out == nil {
		return
	}
	title := ""
	if g != nil {
		title = g.title
	}
	prefix := r.groupPrefix(title)
	if prefix == "" {
		_, _ = fmt.Fprintln(r.out, details)
		return
	}
	_, _ = fmt.Fprintf(r.out, "%s | %s\n", prefix, details)
}

func (r *plainRenderer) errLabel() string {
	return r.plainSprintf("[bold][light_red]ERR[reset]")
}

func (r *plainRenderer) warnLabel() string {
	return r.plainSprintf("[bold][yellow]WARN[reset]")
}

func (r *plainRenderer) renderEvent(now time.Time, e Event, st *engineState) {
	if r == nil || r.out == nil {
		return
	}

	switch e.Type {
	case EventPrintLines:
		for _, line := range e.Lines {
			_, _ = fmt.Fprintln(r.out, line)
		}
	case EventTaskUpdate:
		t := (*taskState)(nil)
		if st != nil {
			t = st.taskByID[e.TaskID]
		}
		if t == nil || t.g == nil {
			return
		}
		r.maybePrintDownloadStart(now, t)
	case EventTaskState:
		t := (*taskState)(nil)
		if st != nil {
			t = st.taskByID[e.TaskID]
		}
		if t == nil || t.g == nil {
			return
		}

		if t.status == taskStatusRunning {
			switch t.kind {
			case taskKindDownload:
				r.maybePrintDownloadStart(now, t)
			default:
				r.maybePrintGenericStart(now, t)
			}
			return
		}
		if t.status == taskStatusRetrying {
			r.printRetry(now, t)
			return
		}
		if t.status == taskStatusError {
			r.printError(now, t)
			return
		}
		if t.status == taskStatusSkipped {
			r.printSkipped(now, t)
			return
		}
		if t.status == taskStatusCanceled {
			r.printCanceled(now, t)
			return
		}
	default:
	}
}

func (r *plainRenderer) maybePrintGenericStart(now time.Time, t *taskState) {
	if r == nil || t == nil || t.plainStartPrinted {
		return
	}
	t.plainStartPrinted = true
	if t.startAt.IsZero() {
		t.startAt = now
	}

	title := r.plainSprintf("[green]%s[reset]", t.title)
	details := ""
	switch {
	case t.meta != "" && t.message != "":
		details = r.plainSprintf("%s [dim]%s[reset] [dim]%s[reset]", title, t.meta, t.message)
	case t.meta != "":
		details = r.plainSprintf("%s [dim]%s[reset]", title, t.meta)
	case t.message != "":
		details = r.plainSprintf("%s [dim]%s[reset]", title, t.message)
	default:
		details = title
	}
	r.printlnWithGroup(t.g, details)
}

func (r *plainRenderer) maybePrintDownloadStart(now time.Time, t *taskState) {
	if r == nil || t == nil || t.downloadStartPrinted || t.kind != taskKindDownload {
		return
	}
	if t.status != taskStatusRunning {
		return
	}
	t.downloadStartPrinted = true
	if t.startAt.IsZero() {
		t.startAt = now
	}

	title := r.plainSprintf("[green]%s[reset]", t.title)
	size := "?"
	if t.total > 0 {
		size = formatBytes(t.total)
	}
	details := ""
	switch {
	case t.meta != "":
		details = r.plainSprintf("%s [dim]%s[reset] [dim](%s)[reset]", title, t.meta, size)
	default:
		details = r.plainSprintf("%s [dim](%s)[reset]", title, size)
	}
	r.printlnWithGroup(t.g, details)
}

func (r *plainRenderer) printRetry(_ time.Time, t *taskState) {
	if r == nil || t == nil {
		return
	}
	label := r.warnLabel()

	title := t.title
	if t.meta != "" {
		title += " " + t.meta
	}
	if t.message != "" {
		r.printlnWithGroup(t.g, fmt.Sprintf("%s - %s: %s", label, title, t.message))
		return
	}
	r.printlnWithGroup(t.g, fmt.Sprintf("%s - %s", label, title))
}

func (r *plainRenderer) printError(_ time.Time, t *taskState) {
	if r == nil || t == nil {
		return
	}

	errLabel := r.errLabel()
	elapsed := t.endAt.Sub(t.startAt)
	title := t.title
	if t.meta != "" {
		title += " " + t.meta
	}
	if t.message != "" {
		r.printlnWithGroup(t.g, fmt.Sprintf("%s - %s: %s (%s)", errLabel, title, t.message, formatDuration(elapsed)))
		return
	}
	r.printlnWithGroup(t.g, fmt.Sprintf("%s - %s (%s)", errLabel, title, formatDuration(elapsed)))
}

func (r *plainRenderer) printSkipped(_ time.Time, t *taskState) {
	if r == nil || t == nil {
		return
	}

	elapsed := t.endAt.Sub(t.startAt)
	title := t.title
	if t.meta != "" {
		title += " " + t.meta
	}
	if t.message != "" {
		r.printlnWithGroup(t.g, fmt.Sprintf("SKIP - %s: %s (%s)", title, t.message, formatDuration(elapsed)))
		return
	}
	r.printlnWithGroup(t.g, fmt.Sprintf("SKIP - %s (%s)", title, formatDuration(elapsed)))
}

func (r *plainRenderer) printCanceled(_ time.Time, t *taskState) {
	if r == nil || t == nil {
		return
	}

	elapsed := t.endAt.Sub(t.startAt)
	title := t.title
	if t.meta != "" {
		title += " " + t.meta
	}
	if t.message != "" {
		r.printlnWithGroup(t.g, fmt.Sprintf("CANCEL - %s: %s (%s)", title, t.message, formatDuration(elapsed)))
		return
	}
	r.printlnWithGroup(t.g, fmt.Sprintf("CANCEL - %s (%s)", title, formatDuration(elapsed)))
}
