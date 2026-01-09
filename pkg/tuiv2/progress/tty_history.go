package progress

import (
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"
)

// maybeSealGroupLocked marks g as sealed when it is finished, and returns the
// final lines that should be appended to the History area in TTY mode.
//
// It must be called with ui.mu held.
func (ui *UI) maybeSealGroupLocked(g *Group) []string {
	if ui == nil || ui.mode != ModeTTY || ui.out == nil || ui.closed || g == nil {
		return nil
	}
	if g.sealed || !g.closed || len(g.tasks) == 0 {
		return nil
	}

	for _, t := range g.tasks {
		if t != nil && t.status == taskStatusRunning {
			return nil
		}
	}

	g.sealed = true

	width := 80
	if ui.out != nil && term.IsTerminal(int(ui.out.Fd())) {
		if w, _, err := term.GetSize(int(ui.out.Fd())); err == nil && w > 0 {
			width = w
		}
	}
	ctx := ttyRenderContext{
		styles: ui.ttyStyles,
		width:  width,
		// There are no running tasks, so the spinner does not matter.
		spinner: "",
	}
	return ttyGroupComponent{group: g}.Lines(ctx, 1_000_000)
}

// sealGroupSnapshotLocked moves g into the immutable History area in TTY mode,
// even if it is still running.
//
// Unlike maybeSealGroupLocked, this method is used to "freeze" an in-progress
// group (e.g. on Ctrl+C) so subsequent output can proceed without the group
// continuing to redraw in the Active area.
//
// It must be called with ui.mu held.
func (ui *UI) sealGroupSnapshotLocked(g *Group) []string {
	if ui == nil || ui.mode != ModeTTY || ui.out == nil || ui.closed || g == nil {
		return nil
	}
	if g.sealed || len(g.tasks) == 0 {
		return nil
	}

	g.sealed = true

	width := 80
	if ui.out != nil && term.IsTerminal(int(ui.out.Fd())) {
		if w, _, err := term.GetSize(int(ui.out.Fd())); err == nil && w > 0 {
			width = w
		}
	}
	ctx := ttyRenderContext{
		styles: ui.ttyStyles,
		width:  width,
		// Freeze a single spinner frame. This matches the semantics of a snapshot:
		// keep a visual hint that some tasks were still running, but stop animating.
		spinner: ui.ttyStyles.spinner.Render("⠦"),
	}
	return ttyGroupComponent{group: g}.Lines(ctx, 1_000_000)
}

// printHistoryLines appends a (possibly multi-line) block to the History area in
// TTY mode.
func (ui *UI) printHistoryLines(lines []string) {
	if ui == nil || len(lines) == 0 {
		return
	}

	ui.mu.Lock()
	mode := ui.mode
	ttySendCh := ui.ttySendCh
	closed := ui.closed
	ui.mu.Unlock()

	if mode != ModeTTY || ttySendCh == nil || closed {
		return
	}

	// Always start at the beginning of the line.
	//
	// The terminal can echo "^C" on SIGINT, which moves the cursor forward by
	// two columns and would otherwise cause Bubble Tea to print history blocks
	// shifted to the right (leaving remnants like a leading "• ").
	msg := tea.Println("\r" + strings.Join(lines, "\n"))()

	defer func() { _ = recover() }()
	ttySendCh <- msg
}

func (ui *UI) printLogLine(line string) {
	if ui == nil {
		return
	}

	ui.mu.Lock()
	mode := ui.mode
	out := ui.out
	ttySendCh := ui.ttySendCh
	ui.mu.Unlock()

	switch mode {
	case ModeTTY:
		if ttySendCh != nil {
			sent := func() (ok bool) {
				defer func() {
					if r := recover(); r != nil {
						ok = false
					}
				}()
				msg := tea.Println("\r" + line + ansi.EraseLineRight)()
				ttySendCh <- msg
				return true
			}()
			if sent {
				return
			}
		}
		// Fallback: if the Bubble Tea program is gone, write through directly.
		if out == nil {
			out = os.Stderr
		}
		_, _ = out.Write([]byte(line + "\n"))
	case ModePlain, ModeOff:
		if out == nil {
			out = os.Stderr
		}
		_, _ = out.Write([]byte(line + "\n"))
	default:
	}
}

// printLogLineForceTTY appends a log line to the History area even after ui.closed
// has been set, as long as the Bubble Tea program is still running.
//
// It is only used during UI.Close() to flush a partial line.
func (ui *UI) printLogLineForceTTY(line string) {
	if ui == nil {
		return
	}

	ui.mu.Lock()
	mode := ui.mode
	ttySendCh := ui.ttySendCh
	ui.mu.Unlock()

	if mode != ModeTTY || ttySendCh == nil {
		return
	}

	msg := tea.Println("\r" + line + ansi.EraseLineRight)()
	defer func() { _ = recover() }()
	ttySendCh <- msg
}
