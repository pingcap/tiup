package progress

import (
	"bytes"
	"io"
	"strings"
	"sync"

	tuiterm "github.com/pingcap/tiup/pkg/tui/term"
)

type uiWriter struct {
	ui *UI

	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *uiWriter) Write(p []byte) (int, error) {
	ui := w.ui
	if ui == nil {
		return len(p), nil
	}

	ui.mu.Lock()
	mode := ui.mode
	out := ui.out
	ui.mu.Unlock()

	// In non-TTY modes, do not do any special handling: preserve the original
	// byte stream.
	if mode != ModeTTY {
		if out == nil {
			return len(p), nil
		}
		return out.Write(p)
	}

	// In TTY mode, treat output as "history lines": split by newline and append
	// them above the Active area via Bubble Tea.
	w.mu.Lock()
	defer w.mu.Unlock()

	n := len(p)
	for len(p) > 0 {
		i := bytes.IndexByte(p, '\n')
		if i < 0 {
			_, _ = w.buf.Write(p)
			break
		}

		_, _ = w.buf.Write(p[:i])
		line := w.buf.String()
		w.buf.Reset()
		line = strings.TrimSuffix(line, "\r")
		ui.printLogLine(line)

		p = p[i+1:]
	}

	return n, nil
}

func (w *uiWriter) flush() {
	ui := w.ui
	if ui == nil {
		return
	}

	w.mu.Lock()
	line := w.buf.String()
	w.buf.Reset()
	w.mu.Unlock()

	line = strings.TrimSuffix(line, "\r")
	if line == "" {
		return
	}
	ui.printLogLineForceTTY(line)
}

func (w *uiWriter) drainBufferedLine() string {
	if w == nil {
		return ""
	}
	w.mu.Lock()
	line := w.buf.String()
	w.buf.Reset()
	w.mu.Unlock()

	line = strings.TrimSuffix(line, "\r")
	return line
}

var _ io.Writer = (*uiWriter)(nil)
var _ tuiterm.ModeProvider = (*uiWriter)(nil)

func (w *uiWriter) TUIMode() tuiterm.OutputMode {
	ui := w.ui
	if ui == nil {
		return tuiterm.OutputMode{}
	}

	ui.mu.Lock()
	mode := ui.outMode
	ui.mu.Unlock()
	return mode
}
