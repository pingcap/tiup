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

	printedGroup bool
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

func (r *plainRenderer) groupHeader(title string) string {
	return r.plainSprintf("[light_magenta]==>[reset] [bold]%s:[reset]", title)
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
	case EventTaskAdd:
		t := (*taskState)(nil)
		if st != nil {
			t = st.taskByID[e.TaskID]
		}
		if t == nil || t.g == nil {
			return
		}
		if t.g.plainBegun {
			return
		}
		if r.printedGroup {
			_, _ = fmt.Fprintln(r.out)
		}
		r.printedGroup = true
		t.g.plainBegun = true
		_, _ = fmt.Fprintln(r.out, r.groupHeader(t.g.title))
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

	switch {
	case t.meta != "" && t.message != "":
		_, _ = fmt.Fprintln(r.out, r.plainSprintf("  [green]+[reset] %s [dim]%s[reset] [dim]%s[reset]", t.title, t.meta, t.message))
	case t.meta != "":
		_, _ = fmt.Fprintln(r.out, r.plainSprintf("  [green]+[reset] %s [dim]%s[reset]", t.title, t.meta))
	case t.message != "":
		_, _ = fmt.Fprintln(r.out, r.plainSprintf("  [green]+[reset] %s [dim]%s[reset]", t.title, t.message))
	default:
		_, _ = fmt.Fprintln(r.out, r.plainSprintf("  [green]+[reset] %s", t.title))
	}
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

	size := "?"
	if t.total > 0 {
		size = formatBytes(t.total)
	}
	switch {
	case t.meta != "":
		_, _ = fmt.Fprintln(r.out, r.plainSprintf("  [green]+[reset] %s [dim]%s[reset] [dim](%s)[reset]", t.title, t.meta, size))
	default:
		_, _ = fmt.Fprintln(r.out, r.plainSprintf("  [green]+[reset] %s [dim](%s)[reset]", t.title, size))
	}
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
		_, _ = fmt.Fprintf(r.out, "%s - %s: %s\n", label, title, t.message)
		return
	}
	_, _ = fmt.Fprintf(r.out, "%s - %s\n", label, title)
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
		_, _ = fmt.Fprintf(r.out, "%s - %s: %s (%s)\n", errLabel, title, t.message, formatDuration(elapsed))
		return
	}
	_, _ = fmt.Fprintf(r.out, "%s - %s (%s)\n", errLabel, title, formatDuration(elapsed))
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
		_, _ = fmt.Fprintf(r.out, "SKIP - %s: %s (%s)\n", title, t.message, formatDuration(elapsed))
		return
	}
	_, _ = fmt.Fprintf(r.out, "SKIP - %s (%s)\n", title, formatDuration(elapsed))
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
		_, _ = fmt.Fprintf(r.out, "CANCEL - %s: %s (%s)\n", title, t.message, formatDuration(elapsed))
		return
	}
	_, _ = fmt.Fprintf(r.out, "CANCEL - %s (%s)\n", title, formatDuration(elapsed))
}
