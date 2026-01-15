package progress

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"
)

type ttyEventMsg struct {
	Event Event
	Ack   chan ttyEventAck
}

type ttyShutdownMsg struct{}

type ttyEventAck struct {
	Prints []string
}

type ttyModel struct {
	ui     *UI
	state  *engineState
	styles ttyStyles

	width  int
	height int

	spinner       spinner.Model
	spinnerActive bool
}

func newTTYModel(ui *UI) ttyModel {
	m := ttyModel{
		ui:    ui,
		state: newEngineState(),
	}
	if ui != nil {
		m.styles = newTTYStyles(ui.out)
		m.spinner = spinner.New(
			spinner.WithSpinner(spinner.MiniDot),
			spinner.WithStyle(m.styles.spinner),
		)
	}
	return m
}

func (m ttyModel) Init() tea.Cmd {
	return tea.ShowCursor
}

func (m ttyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case ttyShutdownMsg:
		return m, tea.Quit
	case ttyEventMsg:
		ui := m.ui
		prints := []string(nil)
		if msg.Ack != nil {
			defer func() {
				msg.Ack <- ttyEventAck{Prints: prints}
			}()
		}
		if ui == nil {
			return m, nil
		}
		e := msg.Event
		now := e.At
		if now.IsZero() && ui.now != nil {
			now = ui.now()
		}

		if ui.eventLog != nil && e.Type != EventSync {
			ui.eventLog.write(now, e)
		}

		if e.Type == EventSync {
			ui.fulfillSync(e.SyncID)
			return m, m.ensureSpinnerTick()
		}

		// PrintLines is a pure output event: it does not affect progress state.
		switch e.Type {
		case EventPrintLines:
			if len(e.Lines) == 0 {
				return m, m.ensureSpinnerTick()
			}
			lines := make([]string, 0, len(e.Lines))
			for _, line := range e.Lines {
				if line == "" {
					lines = append(lines, "\r"+ansi.EraseLineRight)
					continue
				}
				lines = append(lines, "\r"+line+ansi.EraseLineRight)
			}
			prints = append(prints, strings.Join(lines, "\n"))
			return m, m.ensureSpinnerTick()
		default:
		}

		m.state.applyEvent(now, e)

		// Seal snapshots (explicit).
		if e.Type == EventGroupClose && e.Finished != nil && !*e.Finished {
			if g := m.state.groupByID[e.GroupID]; g != nil && g.sealed {
				if lines := m.snapshotLines(g, true); len(lines) > 0 {
					prints = append(prints, "\r"+strings.Join(lines, "\n"))
				}
			}
		}

		// Seal finished groups (auto).
		for _, g := range m.state.groups {
			if g == nil || !g.canAutoSeal() {
				continue
			}
			g.sealed = true
			if lines := m.snapshotLines(g, false); len(lines) > 0 {
				prints = append(prints, "\r"+strings.Join(lines, "\n"))
			}
		}

		return m, m.ensureSpinnerTick()
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if m.state != nil && m.state.hasRunning() {
			m.spinnerActive = true
			return m, cmd
		}
		m.spinnerActive = false
		return m, nil
	default:
		return m, nil
	}
}

func (m *ttyModel) ensureSpinnerTick() tea.Cmd {
	if m == nil || m.state == nil {
		return nil
	}
	if !m.state.hasRunning() {
		m.spinnerActive = false
		return nil
	}
	if m.spinnerActive {
		return nil
	}
	m.spinnerActive = true
	return func() tea.Msg { return m.spinner.Tick() }
}

func (m ttyModel) View() string {
	ui := m.ui
	if ui == nil {
		return ""
	}

	width := m.width
	height := m.height
	if (width <= 0 || height <= 0) && ui.outFile != nil && term.IsTerminal(int(ui.outFile.Fd())) {
		if w, h, err := term.GetSize(int(ui.outFile.Fd())); err == nil {
			if width <= 0 && w > 0 {
				width = w
			}
			if height <= 0 && h > 0 {
				height = h
			}
		}
	}
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}

	maxLines := height - 1
	if maxLines < 3 {
		maxLines = 3
	}

	ctx := ttyRenderContext{
		styles:  m.styles,
		width:   width,
		height:  height,
		spinner: m.spinner.View(),
		now:     ui.now(),
	}

	activeLimit := 1_000_000
	blocks := renderTTYBlocks(m.state, ctx, activeLimit)
	lines := flattenBlocks(blocks)
	if len(lines) == 0 {
		return "\r" + ansi.ResetStyle
	}

	for len(lines)+1 > maxLines && activeLimit > 1 {
		activeLimit /= 2
		if activeLimit < 1 {
			activeLimit = 1
		}
		blocks = renderTTYBlocks(m.state, ctx, activeLimit)
		lines = flattenBlocks(blocks)
	}

	if len(lines)+1 > maxLines && len(blocks) > 0 {
		dropped := 0
		for len(lines)+1 > maxLines && len(blocks) > 1 {
			dropped++
			blocks = blocks[1:]
			lines = flattenBlocks(blocks)
		}
		if dropped > 0 {
			notice := ctx.styles.notice.Render(fmt.Sprintf("… and %d more", dropped))
			lines = append([]string{ctx.styles.clipLine(width, notice)}, lines...)
		}
	}

	lines = append(lines, "")
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}

	return "\r" + strings.Join(lines, "\n")
}

func (m ttyModel) snapshotLines(g *groupState, freezeSpinner bool) []string {
	if g == nil || m.ui == nil {
		return nil
	}
	width := m.width
	if width <= 0 {
		width = 80
		if m.ui.outFile != nil && term.IsTerminal(int(m.ui.outFile.Fd())) {
			if w, _, err := term.GetSize(int(m.ui.outFile.Fd())); err == nil && w > 0 {
				width = w
			}
		}
	}
	sp := ""
	if freezeSpinner {
		sp = m.styles.spinner.Render("⠦")
	}
	ctx := ttyRenderContext{
		styles:  m.styles,
		width:   width,
		spinner: sp,
		now:     m.ui.now(),
	}
	return ttyGroupComponent{group: g}.Lines(ctx, 1_000_000)
}

func (ui *UI) startTTY() {
	if ui == nil || ui.out == nil {
		close(ui.doneCh)
		return
	}

	model := newTTYModel(ui)
	p := tea.NewProgram(
		model,
		tea.WithOutput(ui.out),
		tea.WithInput(nil),
		tea.WithoutSignalHandler(),
		tea.WithFPS(10),
	)
	ui.ttyProgram = p

	go func() {
		defer close(ui.ttyDoneCh)
		_, _ = p.Run()
	}()

	sendEvent := func(e Event) bool {
		ackCh := make(chan ttyEventAck)
		p.Send(ttyEventMsg{Event: e, Ack: ackCh})

		var ack ttyEventAck
		select {
		case ack = <-ackCh:
		case <-ui.ttyDoneCh:
			return false
		}

		for _, block := range ack.Prints {
			select {
			case <-ui.ttyDoneCh:
				return false
			default:
			}
			p.Println(block)
		}
		return true
	}

	go func() {
		defer close(ui.doneCh)
		for {
			select {
			case <-ui.closeCh:
				for {
					select {
					case e := <-ui.eventsCh:
						if !sendEvent(e) {
							return
						}
					default:
						p.Send(ttyShutdownMsg{})
						return
					}
				}
			case <-ui.ttyDoneCh:
				return
			case e := <-ui.eventsCh:
				if !sendEvent(e) {
					return
				}
			}
		}
	}()
}
