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
	ui := (*UI)(nil)
	if w != nil {
		ui = w.ui
	}
	if ui == nil {
		return len(p), nil
	}
	if ui.closed.Load() || ui.mode == ModeOff {
		return len(p), nil
	}

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
		ui.emit(Event{
			Type:   EventPrintLine,
			At:     ui.now(),
			Text:   line,
			Stream: "stdout",
		})

		p = p[i+1:]
	}

	return n, nil
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
	ui := (*UI)(nil)
	if w != nil {
		ui = w.ui
	}
	if ui == nil {
		return tuiterm.OutputMode{}
	}
	return ui.outMode
}
