package progress

import (
	"fmt"
)

// Mode decides how the progress UI renders.
//
// - ModeAuto: choose ModeTTY when output is a TTY, otherwise ModePlain.
// - ModeTTY: dynamic multi-line progress display (ANSI).
// - ModePlain: stable event logs, no ANSI overwrite.
// - ModeOff: no progress output.
type Mode int

const (
	ModeAuto Mode = iota
	ModeTTY
	ModePlain
	ModeOff
)

func (m Mode) String() string {
	switch m {
	case ModeAuto:
		return "auto"
	case ModeTTY:
		return "tty"
	case ModePlain:
		return "plain"
	case ModeOff:
		return "off"
	default:
		return fmt.Sprintf("Mode(%d)", int(m))
	}
}
