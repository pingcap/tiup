package progress

import (
	"io"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

type ttyStyles struct {
	renderer *lipgloss.Renderer

	groupRunningIcon lipgloss.Style
	groupSuccessIcon lipgloss.Style
	groupErrorIcon   lipgloss.Style

	taskSuccessIcon  lipgloss.Style
	taskErrorIcon    lipgloss.Style
	taskSkippedIcon  lipgloss.Style
	taskCanceledIcon lipgloss.Style
	taskPendingIcon  lipgloss.Style
	spinner          lipgloss.Style

	progressFilled lipgloss.Style
	progressTrack  lipgloss.Style

	meta    lipgloss.Style
	message lipgloss.Style

	guideRunning lipgloss.Style
	guideSuccess lipgloss.Style

	notice lipgloss.Style
}

func newTTYStyles(out io.Writer) ttyStyles {
	r := lipgloss.NewRenderer(out, termenv.WithTTY(true))

	gray := lipgloss.ANSIColor(termenv.ANSIBrightBlack)
	green := lipgloss.ANSIColor(termenv.ANSIGreen)
	red := lipgloss.ANSIColor(termenv.ANSIRed)
	yellow := lipgloss.ANSIColor(termenv.ANSIYellow)
	cyan := lipgloss.ANSIColor(termenv.ANSICyan)

	return ttyStyles{
		renderer: r,

		groupRunningIcon: r.NewStyle().Foreground(gray),
		groupSuccessIcon: r.NewStyle().Foreground(green).Bold(true),
		groupErrorIcon:   r.NewStyle().Foreground(red).Bold(true),

		taskSuccessIcon:  r.NewStyle().Foreground(green).Bold(true),
		taskErrorIcon:    r.NewStyle().Foreground(red).Bold(true),
		taskSkippedIcon:  r.NewStyle().Foreground(gray),
		taskCanceledIcon: r.NewStyle().Foreground(yellow).Bold(true),
		taskPendingIcon:  r.NewStyle().Foreground(gray).Faint(true),
		spinner:          r.NewStyle().Foreground(cyan).Bold(true),

		progressFilled: r.NewStyle().Foreground(green),
		// Use a "dimmed" gray for the progress track so it stays readable on
		// both dark and light terminal themes (palette mappings vary widely).
		progressTrack: r.NewStyle().Foreground(gray).Faint(true),

		meta:    r.NewStyle().Foreground(gray),
		message: r.NewStyle().Foreground(gray),

		guideRunning: r.NewStyle().Foreground(gray),
		guideSuccess: r.NewStyle().Foreground(green),

		notice: r.NewStyle().Foreground(gray),
	}
}

func (s ttyStyles) clipLine(width int, line string) string {
	if width <= 0 || line == "" {
		return line
	}
	// Force a single line and clip to terminal width. This avoids us having to do
	// manual rune/ANSI width bookkeeping.
	return s.renderer.NewStyle().Inline(true).MaxWidth(width).Render(line)
}
