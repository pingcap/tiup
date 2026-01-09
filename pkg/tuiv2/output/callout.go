package output

import (
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	tuiterm "github.com/pingcap/tiup/pkg/tui/term"
)

// CalloutStyle selects the visual style used by Callout.
type CalloutStyle int

const (
	// CalloutDefault is the default callout style.
	CalloutDefault CalloutStyle = iota
	// CalloutSucceeded is the success callout style.
	CalloutSucceeded
	// CalloutWarning is the warning callout style.
	CalloutWarning
	// CalloutFailed is the failure callout style.
	CalloutFailed
)

// Callout renders a small status+content block.
//
// In color mode, it uses lipgloss to render ANSI styles (no borders).
// When color is disabled, it emits plain text.
type Callout struct {
	Style      CalloutStyle
	StatusText string
	Content    string
}

func (c Callout) resolvedStatusText() string {
	if c.StatusText != "" {
		return c.StatusText
	}
	switch c.Style {
	case CalloutWarning:
		return "▲ WARNING"
	case CalloutSucceeded:
		return "✔︎ SUCCEEDED"
	case CalloutFailed:
		return "✘ FAILED"
	default:
		return ""
	}
}

// Render renders the callout for the given output writer.
func (c Callout) Render(out io.Writer) string {
	if out == nil {
		out = io.Discard
	}

	statusText := c.resolvedStatusText()
	content := strings.TrimRight(c.Content, "\n")

	if !tuiterm.Resolve(out).Color {
		statusLine := ""
		if statusText != "" {
			statusLine = "==> " + statusText
		}
		switch {
		case statusLine == "" && content == "":
			return ""
		case statusLine == "":
			return content + "\n"
		case content == "":
			return statusLine + "\n"
		default:
			return statusLine + "\n\n" + content + "\n"
		}
	}

	r := lipgloss.NewRenderer(out, termenv.WithTTY(true))
	bg := lipgloss.ANSIColor(termenv.ANSIWhite)
	switch c.Style {
	case CalloutSucceeded:
		bg = lipgloss.ANSIColor(termenv.ANSIGreen)
	case CalloutWarning:
		bg = lipgloss.ANSIColor(termenv.ANSIYellow)
	case CalloutFailed:
		bg = lipgloss.ANSIColor(termenv.ANSIRed)
	}

	statusLine := ""
	if statusText != "" {
		statusLine = ansi.ResetStyle + r.NewStyle().
			Background(bg).
			Foreground(lipgloss.ANSIColor(termenv.ANSIBlack)).
			Render(" "+statusText+" ")
	}

	switch {
	case statusLine == "" && content == "":
		return ""
	case statusLine == "":
		return content + "\n"
	case content == "":
		return statusLine + "\n"
	default:
		// Always leave an empty line between the title and body to improve
		// readability and copy/paste behavior.
		return statusLine + "\n\n" + content + "\n"
	}
}
