package progress

import (
	"fmt"

	"github.com/pingcap/tiup/pkg/tui/colorstr"
)

func (ui *UI) plainGroupHeader(title string) string {
	return ui.plainSprintf("[bold][light_magenta]==> %s[reset]", title)
}

func (ui *UI) plainErrorLabel() string {
	return ui.plainSprintf("[bold][light_red]ERR[reset]")
}

func (ui *UI) plainSprintf(format string, args ...any) string {
	tokens := colorstr.DefaultTokens
	tokens.Disable = ui == nil || !ui.outMode.Color
	return tokens.Sprintf(format, args...)
}

func (ui *UI) printPlainGroupHeaderLocked(title string) {
	if ui == nil || ui.mode != ModePlain {
		return
	}

	if ui.plainPrintedGroup {
		ui.printPlainLineLocked("")
	}
	ui.plainPrintedGroup = true
	ui.printPlainLineLocked(ui.plainGroupHeader(title))
}

func (ui *UI) printPlainLineLocked(format string, args ...any) {
	if ui == nil || ui.out == nil || ui.mode != ModePlain {
		return
	}
	if len(args) == 0 {
		_, _ = fmt.Fprintln(ui.out, format)
		return
	}
	_, _ = fmt.Fprintf(ui.out, format+"\n", args...)
}

func (ui *UI) printPlainTaskStartLocked(t *Task) {
	if ui == nil || ui.mode != ModePlain || t == nil {
		return
	}

	title := t.title
	if t.meta != "" {
		title += " " + t.meta
	}
	if t.message != "" {
		title += " " + t.message
	}
	ui.printPlainLineLocked(ui.plainSprintf(" [green]+[reset] %s", title))
}

func (ui *UI) printPlainTaskDoneLocked(t *Task) {
	if ui == nil || ui.mode != ModePlain || t == nil {
		return
	}
	// Plain mode is designed to be compact and stable (append-only logs).
	// We only emit task start events by default.
}

func (ui *UI) printPlainTaskErrorLocked(t *Task) {
	if ui == nil || ui.mode != ModePlain || t == nil {
		return
	}

	errLabel := ui.plainErrorLabel()

	elapsed := t.endAt.Sub(t.startAt)
	title := t.title
	if t.meta != "" {
		title += " " + t.meta
	}
	if t.message != "" {
		ui.printPlainLineLocked("%s - %s: %s (%s)", errLabel, title, t.message, formatDuration(elapsed))
		return
	}
	ui.printPlainLineLocked("%s - %s (%s)", errLabel, title, formatDuration(elapsed))
}

func (ui *UI) printPlainTaskSkippedLocked(t *Task) {
	if ui == nil || ui.mode != ModePlain || t == nil {
		return
	}

	elapsed := t.endAt.Sub(t.startAt)
	title := t.title
	if t.meta != "" {
		title += " " + t.meta
	}
	if t.message != "" {
		ui.printPlainLineLocked("SKIP - %s: %s (%s)", title, t.message, formatDuration(elapsed))
		return
	}
	ui.printPlainLineLocked("SKIP - %s (%s)", title, formatDuration(elapsed))
}

func (ui *UI) printPlainTaskCanceledLocked(t *Task) {
	if ui == nil || ui.mode != ModePlain || t == nil {
		return
	}

	elapsed := t.endAt.Sub(t.startAt)
	title := t.title
	if t.meta != "" {
		title += " " + t.meta
	}
	if t.message != "" {
		ui.printPlainLineLocked("CANCEL - %s: %s (%s)", title, t.message, formatDuration(elapsed))
		return
	}
	ui.printPlainLineLocked("CANCEL - %s (%s)", title, formatDuration(elapsed))
}

func (ui *UI) printPlainDownloadStartLocked(t *Task) {
	if ui == nil || ui.mode != ModePlain || t == nil {
		return
	}

	size := "?"
	if t.total > 0 {
		size = formatBytes(t.total)
	}
	title := t.title
	if t.meta != "" {
		title += " " + t.meta
	}
	ui.printPlainLineLocked(ui.plainSprintf(" [green]+[reset] %s (%s)", title, size))
}
