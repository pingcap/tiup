package term

import (
	"io"
	"os"

	xterm "golang.org/x/term"
)

// Environment variables to control TUI output behavior.
//
// Precedence (highest first):
//  1. NO_COLOR    -> disable color + control sequences
//  2. FORCE_TTY   -> enable color + control sequences
//  3. FORCE_COLOR -> enable color, control sequences follow TTY detection
const (
	EnvNoColor    = "NO_COLOR"
	EnvForceColor = "FORCE_COLOR"
	EnvForceTTY   = "FORCE_TTY"
)

// OutputMode controls what kind of output is allowed for a writer.
//
// Color: whether ANSI styling (colors/bold) is allowed.
// Control: whether ANSI control sequences that rewrite the terminal output
// (spinners, multi-line live updates) are allowed.
type OutputMode struct {
	Color   bool
	Control bool
}

// ModeProvider allows a writer to declare its effective output mode.
//
// This is useful for writer wrappers that preserve the underlying terminal
// capability (e.g. progress UI writers).
type ModeProvider interface {
	TUIMode() OutputMode
}

// Resolve resolves the effective output mode for the given writer.
func Resolve(out io.Writer) OutputMode {
	if out == nil {
		return resolveFile(nil)
	}
	if p, ok := out.(ModeProvider); ok && p != nil {
		return p.TUIMode()
	}
	if f, ok := out.(*os.File); ok {
		return resolveFile(f)
	}
	// Unknown writers are treated as non-TTY by default.
	return resolveFile(nil)
}

// ResolveFile is like Resolve but specialized for *os.File.
func ResolveFile(out *os.File) OutputMode {
	return resolveFile(out)
}

func resolveFile(out *os.File) OutputMode {
	if os.Getenv(EnvNoColor) != "" {
		return OutputMode{}
	}

	if os.Getenv(EnvForceTTY) != "" {
		return OutputMode{Color: true, Control: true}
	}

	isTTY := out != nil && xterm.IsTerminal(int(out.Fd()))

	if os.Getenv(EnvForceColor) != "" {
		return OutputMode{Color: true, Control: isTTY}
	}

	if isTTY {
		return OutputMode{Color: true, Control: true}
	}
	return OutputMode{}
}
