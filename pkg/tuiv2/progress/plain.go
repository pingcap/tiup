package progress

import (
	"fmt"
	"strings"

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

	verb := "Running"
	if g := t.g; g != nil {
		if fields := strings.Fields(g.title); len(fields) > 0 {
			verb = fields[0]
		}
	}

	title := t.title
	if t.meta != "" {
		title += " " + t.meta
	}
	if t.message != "" {
		title += " " + t.message
	}
	ui.printPlainLineLocked("%s %s", verb, title)
}

func (ui *UI) printPlainTaskDoneLocked(t *Task) {
	if ui == nil || ui.mode != ModePlain || t == nil {
		return
	}

	// Generic tasks already emitted a start event in plain mode.
	// Keep the log compact by only printing completion details for downloads.
	if t.kind != taskKindDownload {
		return
	}

	elapsed := t.endAt.Sub(t.startAt)
	title := t.title
	if t.meta != "" {
		title += " " + t.meta
	}
	switch t.kind {
	case taskKindDownload:
		size := t.total
		if size <= 0 {
			size = t.current
		}
		ui.printPlainLineLocked("Downloaded  %s (%s, %s, %s)", title, formatBytes(size), formatDuration(elapsed), formatSpeed(t.speedBps))
	default:
		ui.printPlainLineLocked("OK  - %s (%s)", title, formatDuration(elapsed))
	}
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
	ui.printPlainLineLocked("Downloading %s (%s)", title, size)
}
