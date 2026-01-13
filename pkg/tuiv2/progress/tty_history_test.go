package progress

import (
	"io"
	"os"
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func teaPrintLineBody(t *testing.T, msg tea.Msg) string {
	t.Helper()

	v := reflect.ValueOf(msg)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	require.Equal(t, reflect.Struct, v.Kind(), "unexpected msg type: %T", msg)

	body := v.FieldByName("messageBody")
	require.True(t, body.IsValid(), "unexpected msg type: %T", msg)
	require.Equal(t, reflect.String, body.Kind(), "unexpected msg type: %T", msg)
	return body.String()
}

func TestPrintLogLine_TTYStartsAtLineBeginningAndErasesRight(t *testing.T) {
	ui := &UI{
		mode:      ModeTTY,
		ttySendCh: make(chan tea.Msg, 1),
	}

	ui.printLogLine("hello")

	msg := <-ui.ttySendCh
	body := teaPrintLineBody(t, msg)
	require.Equal(t, "\rhello"+ansi.EraseLineRight, body)
}

func TestPrintHistoryLines_TTYStartsAtLineBeginning(t *testing.T) {
	ui := &UI{
		mode:      ModeTTY,
		ttySendCh: make(chan tea.Msg, 1),
	}

	ui.printHistoryLines([]string{"a", "b"})

	msg := <-ui.ttySendCh
	body := teaPrintLineBody(t, msg)
	require.Equal(t, "\ra\nb", body)
}

func TestUIWriter_TTYBuffersUntilNewline(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })
	t.Cleanup(func() { _ = w.Close() })

	ui := &UI{out: w, mode: ModeTTY}
	uw := &uiWriter{ui: ui}
	ui.writer = uw

	_, err = io.WriteString(ui.Writer(), "hello")
	require.NoError(t, err)
	_, err = io.WriteString(ui.Writer(), " world\r\n")
	require.NoError(t, err)

	_ = w.Close()
	got, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, "hello world\n", string(got))
}

func TestUIClose_TTYFlushesBufferedLineToHistory(t *testing.T) {
	ui := &UI{
		mode:      ModeTTY,
		ttySendCh: make(chan tea.Msg, 1),
	}
	uw := &uiWriter{ui: ui}
	ui.writer = uw

	_, err := io.WriteString(ui.Writer(), "hello")
	require.NoError(t, err)
	require.NoError(t, ui.Close())

	msg, ok := <-ui.ttySendCh
	require.True(t, ok, "expected flushed history message")
	body := teaPrintLineBody(t, msg)
	require.Equal(t, "\rhello"+ansi.EraseLineRight, body)
}
