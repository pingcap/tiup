package progress

import (
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	tuiterm "github.com/pingcap/tiup/pkg/tui/term"
)

// Options controls how the progress UI behaves.
type Options struct {
	// Mode decides how the progress UI renders.
	Mode Mode
	// Out is the output writer used by the UI.
	//
	// If nil, it defaults to os.Stderr.
	//
	// ModeAuto and ModeTTY rely on TTY detection. Only *os.File writers can be
	// detected as TTY; other writers will be treated as non-TTY and fall back to
	// plain output.
	Out io.Writer

	// EventLog is an optional JSON-lines sink of the event stream.
	//
	// It is primarily intended for daemon mode: the daemon process writes event
	// logs to a file, and the starter process replays them in a real TTY.
	EventLog io.Writer

	// Now returns the current time.
	// If nil, it defaults to time.Now.
	//
	// It exists to make tests deterministic.
	Now func() time.Time
}

// UI is a unified progress display for both TTY users and non-TTY logs/CI.
//
// Create a UI via New, then create Group/Task objects and update them from any goroutine.
// Call Close when the program exits.
type UI struct {
	out     io.Writer
	outFile *os.File
	mode    Mode
	outMode tuiterm.OutputMode

	now func() time.Time

	closed atomic.Bool
	nextID atomic.Uint64

	syncMu      sync.Mutex
	syncWaiters map[uint64]chan struct{}

	eventsCh chan Event
	closeCh  chan struct{}
	doneCh   chan struct{}

	writer *uiWriter

	ttyProgram *tea.Program
	ttyDoneCh  chan struct{}

	plainDoneCh chan struct{}

	eventLog *eventLogSink
}

const defaultEventBuffer = 4096

// New creates a new progress UI.
func New(opts Options) *UI {
	out := opts.Out
	if out == nil {
		out = os.Stderr
	}
	outFile, _ := out.(*os.File)

	now := opts.Now
	if now == nil {
		now = time.Now
	}

	requested := opts.Mode
	termCap := tuiterm.Resolve(out)

	actual := resolveMode(requested, termCap)
	if actual == ModeTTY && outFile == nil {
		// Can't run TTY mode without a real file descriptor.
		actual = ModePlain
	}
	termCap.Control = actual == ModeTTY

	ui := &UI{
		out:     out,
		outFile: outFile,
		mode:    actual,
		outMode: termCap,
		now:     now,

		eventsCh: make(chan Event, defaultEventBuffer),
		closeCh:  make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
	ui.writer = &uiWriter{ui: ui}

	if opts.EventLog != nil {
		ui.eventLog = newEventLogSink(opts.EventLog)
	}

	switch actual {
	case ModeTTY:
		ui.ttyDoneCh = make(chan struct{})
		ui.startTTY()
	case ModePlain:
		ui.plainDoneCh = make(chan struct{})
		go ui.runPlain()
	case ModeOff:
		close(ui.doneCh)
	default:
		ui.plainDoneCh = make(chan struct{})
		go ui.runPlain()
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
	return ui.mode
}

// Close stops the UI and releases any internal resources.
func (ui *UI) Close() error {
	if ui == nil {
		return nil
	}
	if !ui.closed.CompareAndSwap(false, true) {
		return nil
	}

	// Flush any pending partial line before stopping the engine.
	if ui.writer != nil {
		if line := ui.writer.drainBufferedLine(); line != "" {
			ui.emitForced(Event{
				Type:  EventPrintLines,
				At:    ui.now(),
				Lines: []string{line},
			})
		}
	}

	close(ui.closeCh)

	switch ui.mode {
	case ModeTTY:
		if ui.ttyDoneCh != nil {
			<-ui.ttyDoneCh
		}
	case ModePlain:
		if ui.plainDoneCh != nil {
			<-ui.plainDoneCh
		}
	default:
	}

	<-ui.doneCh
	return nil
}

// Group creates a new group of tasks (usually a "stage") under this UI.
func (ui *UI) Group(title string) *Group {
	if ui == nil || ui.closed.Load() {
		return &Group{ui: nil, title: title}
	}
	id := ui.nextID.Add(1)
	g := &Group{ui: ui, id: id, title: title}
	t := title
	ui.emit(Event{
		Type:    EventGroupAdd,
		At:      ui.now(),
		GroupID: id,
		Title:   &t,
	})
	return g
}

// Writer returns a writer that is safe to use together with the progress UI.
//
// In ModeTTY, it appends complete lines to the History area (above the Active
// area), so they remain in the terminal scrollback and don't corrupt multi-line
// progress rendering.
//
// In ModePlain and ModeOff, it still emits line-based events so daemon mode can
// persist and replay output.
func (ui *UI) Writer() io.Writer {
	if ui == nil {
		return io.Discard
	}
	return ui.writer
}

// Sync blocks until all previously emitted events are processed by the UI engine.
//
// It is primarily intended for daemon mode: callers may use it to ensure output
// is fully persisted to the event log before exposing readiness signals (e.g.
// creating the HTTP command server port file).
func (ui *UI) Sync() {
	if ui == nil || ui.closed.Load() || ui.mode == ModeOff {
		return
	}

	// Flush any pending partial line before syncing.
	if ui.writer != nil {
		if line := ui.writer.drainBufferedLine(); line != "" {
			ui.emit(Event{
				Type:  EventPrintLines,
				At:    ui.now(),
				Lines: []string{line},
			})
		}
	}

	id := ui.nextID.Add(1)
	waitCh := make(chan struct{})

	ui.syncMu.Lock()
	if ui.syncWaiters == nil {
		ui.syncWaiters = make(map[uint64]chan struct{})
	}
	ui.syncWaiters[id] = waitCh
	ui.syncMu.Unlock()

	defer ui.removeSyncWaiter(id)

	e := Event{
		Type:   EventSync,
		At:     ui.now(),
		SyncID: id,
	}

	select {
	case <-ui.closeCh:
		return
	case ui.eventsCh <- e:
	}

	select {
	case <-waitCh:
	case <-ui.doneCh:
	case <-ui.closeCh:
	}
}

func (ui *UI) removeSyncWaiter(id uint64) {
	if ui == nil || id == 0 {
		return
	}
	ui.syncMu.Lock()
	delete(ui.syncWaiters, id)
	ui.syncMu.Unlock()
}

func (ui *UI) fulfillSync(id uint64) {
	if ui == nil || id == 0 {
		return
	}
	ui.syncMu.Lock()
	waitCh := ui.syncWaiters[id]
	delete(ui.syncWaiters, id)
	ui.syncMu.Unlock()
	if waitCh != nil {
		close(waitCh)
	}
}

// PrintLines prints one or more text lines as a single output block.
//
// It flushes any pending partial line from UI.Writer() first, then appends an
// output block.
func (ui *UI) PrintLines(lines []string) {
	if ui == nil || ui.closed.Load() {
		return
	}
	if len(lines) == 0 && ui.writer == nil {
		return
	}

	outLines := make([]string, 0, len(lines)+1)
	if ui.writer != nil {
		if line := ui.writer.drainBufferedLine(); line != "" {
			outLines = append(outLines, line)
		}
	}
	outLines = append(outLines, lines...)
	if len(outLines) == 0 {
		return
	}

	ui.emit(Event{
		Type:  EventPrintLines,
		At:    ui.now(),
		Lines: outLines,
	})
}

func resolveMode(requested Mode, termCap tuiterm.OutputMode) Mode {
	if requested == ModeOff {
		return ModeOff
	}
	if requested == ModePlain {
		return ModePlain
	}
	if requested == ModeTTY {
		if termCap.Control {
			return ModeTTY
		}
		return ModePlain
	}

	// ModeAuto: if we can do terminal output rewriting, use TTY mode.
	if termCap.Control {
		return ModeTTY
	}
	return ModePlain
}

func (ui *UI) emit(e Event) {
	if ui == nil || ui.closed.Load() {
		return
	}
	if ui.mode == ModeOff {
		return
	}
	if e.At.IsZero() && ui.now != nil {
		e.At = ui.now()
	}

	select {
	case <-ui.closeCh:
		return
	default:
	}
	select {
	case ui.eventsCh <- e:
	case <-ui.closeCh:
	}
}

func (ui *UI) emitForced(e Event) {
	if ui == nil {
		return
	}
	if ui.mode == ModeOff {
		return
	}
	if e.At.IsZero() && ui.now != nil {
		e.At = ui.now()
	}

	select {
	case <-ui.closeCh:
		return
	default:
	}
	select {
	case ui.eventsCh <- e:
	case <-ui.closeCh:
	}
}

func (ui *UI) runPlain() {
	defer func() {
		if ui.plainDoneCh != nil {
			close(ui.plainDoneCh)
		}
		close(ui.doneCh)
	}()

	if ui.mode == ModeOff || ui.out == nil {
		<-ui.closeCh
		return
	}

	st := newEngineState()
	r := newPlainRenderer(ui.out, ui.outMode)

	for {
		select {
		case <-ui.closeCh:
			for {
				select {
				case e := <-ui.eventsCh:
					ui.processPlainEvent(e, st, r)
				default:
					return
				}
			}
		case e := <-ui.eventsCh:
			ui.processPlainEvent(e, st, r)
		}
	}
}

func (ui *UI) processPlainEvent(e Event, st *engineState, r *plainRenderer) {
	now := e.At
	if now.IsZero() {
		now = ui.now()
	}

	if ui.eventLog != nil && e.Type != EventSync {
		ui.eventLog.write(now, e)
	}

	if e.Type == EventSync {
		ui.fulfillSync(e.SyncID)
		return
	}

	st.applyEvent(now, e)
	r.renderEvent(now, e, st)
}

// DecodeEvent decodes a single JSON event line.
func DecodeEvent(line []byte) (Event, error) {
	return parseEventLine(line)
}

// ReplayEvent injects a single Event into this UI.
//
// It is intended for daemon mode starter processes that tail an event log file.
func (ui *UI) ReplayEvent(e Event) {
	if ui == nil {
		return
	}
	ui.emit(e)
}
