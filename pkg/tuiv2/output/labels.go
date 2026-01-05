package output

import (
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	tuiterm "github.com/pingcap/tiup/pkg/tui/term"
)

// Labels renders aligned two-column text (label/value pairs).
//
// When color is enabled, the label column is styled as dark gray and the value
// column keeps the default terminal style.
//
// The value column is intentionally not width-constrained. Long values (e.g.
// URLs) are expected to wrap naturally by the terminal.
type Labels struct {
	Rows [][2]string

	// LabelWidth pads the label column to the given width.
	// When 0, it is computed from Rows.
	LabelWidth int

	// Gap is the number of spaces between the two columns.
	// When 0, it defaults to 1.
	Gap int
}

func (l Labels) Lines(out io.Writer) []string {
	if out == nil {
		out = io.Discard
	}
	if len(l.Rows) == 0 {
		return nil
	}

	gap := l.Gap
	if gap <= 0 {
		gap = 1
	}
	sep := strings.Repeat(" ", gap)

	width := l.LabelWidth
	if width < 0 {
		width = 0
	}
	if width == 0 {
		for _, row := range l.Rows {
			if w := ansi.StringWidth(row[0]); w > width {
				width = w
			}
		}
	}

	padRight := func(s string) string {
		if width <= 0 {
			return s
		}
		w := ansi.StringWidth(s)
		if w >= width {
			return s
		}
		return s + strings.Repeat(" ", width-w)
	}

	if !tuiterm.Resolve(out).Color {
		lines := make([]string, 0, len(l.Rows))
		for _, row := range l.Rows {
			label := padRight(row[0])
			value := row[1]
			if label == "" && value == "" {
				lines = append(lines, "")
				continue
			}
			if value == "" {
				lines = append(lines, label)
				continue
			}
			lines = append(lines, label+sep+value)
		}
		return lines
	}

	r := lipgloss.NewRenderer(out, termenv.WithTTY(true))
	labelStyle := r.NewStyle().Foreground(lipgloss.ANSIColor(termenv.ANSIBrightBlack))

	lines := make([]string, 0, len(l.Rows))
	for _, row := range l.Rows {
		label := padRight(row[0])
		value := row[1]
		if label == "" && value == "" {
			lines = append(lines, "")
			continue
		}
		if value == "" {
			lines = append(lines, ansi.ResetStyle+labelStyle.Render(label))
			continue
		}
		lines = append(lines, ansi.ResetStyle+labelStyle.Render(label)+sep+value)
	}
	return lines
}

func (l Labels) Render(out io.Writer) string {
	return strings.Join(l.Lines(out), "\n")
}
