package progress

import (
	"io"
	"os"
	"sync"

	tea "github.com/charmbracelet/bubbletea"

	tuiterm "github.com/pingcap/tiup/pkg/tui/term"
)

// Options controls how the progress UI behaves.
type Options struct {
	// Mode decides how the progress UI renders.
	Mode Mode
	// Out is the output file used by the UI.
	// If nil, it defaults to os.Stderr.
	Out *os.File
}

// UI is a unified progress display for both TTY users and non-TTY logs/CI.
//
// Create a UI via New, then create Group/Task objects and update them from any goroutine.
// Call Close when the program exits.
type UI struct {
	out     *os.File
	mode    Mode
	outMode tuiterm.OutputMode

	mu     sync.Mutex
	closed bool

	groups []*Group

	// Bubble Tea TTY program state.
	ttyProgram *tea.Program
	ttySendCh  chan tea.Msg
	ttyDoneCh  chan struct{}
	ttyStyles  ttyStyles

	writer io.Writer
}

// New creates a new progress UI.
func New(opts Options) *UI {
	out := opts.Out
	if out == nil {
		out = os.Stderr
	}

	requested := opts.Mode
	cap := tuiterm.ResolveFile(out)

	actual := resolveMode(requested, cap)
	cap.Control = actual == ModeTTY
	ui := &UI{
		out:     out,
		mode:    actual,
		outMode: cap,
		writer:  &uiWriter{},
	}
	ui.writer.(*uiWriter).ui = ui

	if actual == ModeTTY {
		ui.ttySendCh = make(chan tea.Msg, 1024)
		ui.ttyDoneCh = make(chan struct{})
		ui.ttyStyles = newTTYStyles(out)
		ui.startTTY()
	}

	return ui
}

// Mode returns the resolved mode used by this UI.
//
// It may differ from Options.Mode when Options.Mode is ModeAuto (or when
// terminal capability checks force a downgrade to ModePlain).
func (ui *UI) Mode() Mode {
	if ui == nil {
		return ModeOff
	}
	ui.mu.Lock()
	defer ui.mu.Unlock()
	return ui.mode
}

// Close stops the UI and releases any internal resources.
func (ui *UI) Close() error {
	if ui == nil {
		return nil
	}

	ui.mu.Lock()
	if ui.closed {
		ui.mu.Unlock()
		return nil
	}
	ui.closed = true
	mode := ui.mode
	ttyDoneCh := ui.ttyDoneCh
	ttySendCh := ui.ttySendCh
	ttyProgram := ui.ttyProgram
	ui.mu.Unlock()

	if mode == ModeTTY {
		// Flush any pending partial line from the UI writer to avoid losing the
		// last line when shutting down.
		if w, ok := ui.writer.(*uiWriter); ok {
			w.flush()
		}

		// Ask the Bubble Tea program to quit.
		// Quit is sent via ttySendCh so it is processed after any previously
		// enqueued history prints.
		if ttyProgram != nil {
			if ttySendCh != nil {
				func() {
					// Best-effort: channel may already be closed or blocked if the program
					// exited unexpectedly.
					defer func() { _ = recover() }()
					select {
					case ttySendCh <- tea.Quit():
					default:
						ttyProgram.Quit()
					}
				}()
			} else {
				ttyProgram.Quit()
			}
		}
		if ttyDoneCh != nil {
			<-ttyDoneCh
		}

		// Stop the sender loop after the Bubble Tea program has fully exited.
		// This avoids a window where direct writes can interleave with the
		// still-running renderer.
		if ttySendCh != nil {
			func() {
				defer func() { _ = recover() }()
				close(ttySendCh)
			}()
		}
	}

	return nil
}

// Group creates a new group of tasks (usually a "stage") under this UI.
func (ui *UI) Group(title string) *Group {
	if ui == nil {
		return &Group{ui: nil, title: title}
	}

	g := &Group{ui: ui, title: title, showMeta: true}
	ui.mu.Lock()
	if ui.closed {
		ui.mu.Unlock()
		return &Group{ui: nil, title: title}
	}
	ui.groups = append(ui.groups, g)
	ui.markDirtyLocked()
	ui.mu.Unlock()
	return g
}

// Writer returns a writer that is safe to use together with the progress UI.
//
// In ModeTTY, it appends complete lines to the Bubble Tea History area (above the
// Active area), so they remain in the terminal scrollback and don't corrupt
// multi-line progress rendering.
//
// In ModePlain and ModeOff, it writes directly to the underlying output.
func (ui *UI) Writer() io.Writer {
	if ui == nil {
		return io.Discard
	}
	return ui.writer
}

func resolveMode(requested Mode, cap tuiterm.OutputMode) Mode {
	if requested == ModeOff {
		return ModeOff
	}
	if requested == ModePlain {
		return ModePlain
	}
	if requested == ModeTTY {
		if cap.Control {
			return ModeTTY
		}
		return ModePlain
	}

	// ModeAuto: if we can do terminal output rewriting, use TTY mode.
	if cap.Control {
		return ModeTTY
	}
	return ModePlain
}

func (ui *UI) markDirtyLocked() {
	if ui == nil || ui.mode != ModeTTY || ui.ttySendCh == nil || ui.closed {
		return
	}
	defer func() { _ = recover() }()
	select {
	case ui.ttySendCh <- ttyDirtyMsg{}:
	default:
	}
}
