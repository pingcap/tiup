package progress

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"
)

// ttyDirtyMsg tells the Bubble Tea program that the progress state has changed and
// it should re-render the Active area.
type ttyDirtyMsg struct{}

type ttyModel struct {
	ui *UI

	width  int
	height int

	spinner       spinner.Model
	spinnerActive bool
}

func newTTYModel(ui *UI) ttyModel {
	m := ttyModel{ui: ui}
	if ui != nil {
		m.spinner = spinner.New(
			spinner.WithSpinner(spinner.MiniDot),
			spinner.WithStyle(ui.ttyStyles.spinner),
		)
	}
	return m
}

func (m ttyModel) Init() tea.Cmd {
	// Keep the terminal cursor visible. Bubble Tea hides it by default, which is
	// common for fullscreen TUIs but undesirable for an inline progress display:
	// if the program crashes, the terminal can be left in a "cursor hidden" state.
	return func() tea.Msg { return tea.ShowCursor() }
}

func (m ttyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case ttyDirtyMsg:
		if m.ui != nil {
			m.ui.mu.Lock()
			hasRunning := m.ui.hasRunningLocked()
			m.ui.mu.Unlock()
			if hasRunning {
				if !m.spinnerActive {
					m.spinnerActive = true
					return m, func() tea.Msg { return m.spinner.Tick() }
				}
				return m, nil
			}
		}
		m.spinnerActive = false
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if m.ui != nil {
			m.ui.mu.Lock()
			hasRunning := m.ui.hasRunningLocked()
			m.ui.mu.Unlock()
			if hasRunning {
				m.spinnerActive = true
				return m, cmd
			}
		}
		m.spinnerActive = false
		return m, nil
	default:
		return m, nil
	}
}

func (m ttyModel) View() string {
	ui := m.ui
	if ui == nil {
		return ""
	}

	width := m.width
	height := m.height
	if (width <= 0 || height <= 0) && ui.out != nil && term.IsTerminal(int(ui.out.Fd())) {
		if w, h, err := term.GetSize(int(ui.out.Fd())); err == nil {
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

	ui.mu.Lock()
	defer ui.mu.Unlock()

	ctx := ttyRenderContext{
		styles:  ui.ttyStyles,
		width:   width,
		height:  height,
		spinner: m.spinner.View(),
	}

	activeLimit := 1_000_000

	blocks := renderTTYBlocksLocked(ui.groups, ctx, activeLimit)
	lines := flattenBlocks(blocks)
	if len(lines) == 0 {
		// Bubble Tea replaces an empty string view with a literal space to
		// "render nothing". That space shows up as a confusing blank line at the
		// beginning of the output in some shells. Render a no-op ANSI reset
		// sequence instead so the active area can still be cleared without
		// printing visible characters.
		return "\r" + ansi.ResetStyle
	}
	for len(lines)+1 > maxLines && activeLimit > 1 {
		activeLimit /= 2
		if activeLimit < 1 {
			activeLimit = 1
		}
		blocks = renderTTYBlocksLocked(ui.groups, ctx, activeLimit)
		lines = flattenBlocks(blocks)
	}

	// Still too long: drop oldest groups (keep the latest ones).
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

	// Reserve one last empty line for the cursor.
	lines = append(lines, "")

	if maxLines > 0 && len(lines) > maxLines {
		// Best-effort fallback: keep the newest lines to avoid terminal scrolling.
		lines = lines[len(lines)-maxLines:]
	}

	// Always start at the beginning of the line.
	//
	// Some terminals echo "^C" on SIGINT, which moves the cursor forward by two
	// columns. If we don't reset, Bubble Tea will redraw the active area shifted
	// to the right, leaving remnants like a leading "• ".
	return "\r" + strings.Join(lines, "\n")
}

func (ui *UI) startTTY() {
	if ui == nil || ui.out == nil || ui.ttySendCh == nil || ui.ttyDoneCh == nil {
		return
	}

	model := newTTYModel(ui)
	p := tea.NewProgram(
		model,
		tea.WithOutput(ui.out),     // default: os.Stdout
		tea.WithInput(nil),         // do not read stdin or enter raw mode
		tea.WithoutSignalHandler(), // signals are handled by the main program
		tea.WithFPS(10),            // cap redraw rate
	)
	ui.ttyProgram = p

	// Run Bubble Tea in a goroutine so the caller can keep doing work.
	go func() {
		defer close(ui.ttyDoneCh)
		_, _ = p.Run()
	}()

	// Forward messages through a buffered channel to avoid blocking callers on
	// Program.Send before the program loop is ready.
	go func() {
		for {
			select {
			case msg, ok := <-ui.ttySendCh:
				if !ok {
					return
				}
				p.Send(msg)
			case <-ui.ttyDoneCh:
				return
			}
		}
	}()

	// Force an initial render.
	select {
	case ui.ttySendCh <- ttyDirtyMsg{}:
	default:
	}
}

func (ui *UI) hasRunningLocked() bool {
	for _, g := range ui.groups {
		if g == nil || g.sealed {
			continue
		}
		for _, t := range g.tasks {
			if t != nil && t.status == taskStatusRunning {
				return true
			}
		}
	}
	return false
}
