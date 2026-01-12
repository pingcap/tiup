package progress

import (
	"io"
	"os"
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func teaPrintLineBody(t *testing.T, msg tea.Msg) string {
	t.Helper()

	v := reflect.ValueOf(msg)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		t.Fatalf("unexpected msg type: %T", msg)
	}

	body := v.FieldByName("messageBody")
	if !body.IsValid() || body.Kind() != reflect.String {
		t.Fatalf("unexpected msg type: %T", msg)
	}
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
	if want := "\rhello" + ansi.EraseLineRight; body != want {
		t.Fatalf("unexpected msg body:\nwant %q\ngot  %q", want, body)
	}
}

func TestPrintHistoryLines_TTYStartsAtLineBeginning(t *testing.T) {
	ui := &UI{
		mode:      ModeTTY,
		ttySendCh: make(chan tea.Msg, 1),
	}

	ui.printHistoryLines([]string{"a", "b"})

	msg := <-ui.ttySendCh
	body := teaPrintLineBody(t, msg)
	if want := "\ra\nb"; body != want {
		t.Fatalf("unexpected msg body:\nwant %q\ngot  %q", want, body)
	}
}

func TestUIWriter_TTYBuffersUntilNewline(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer func() { _ = r.Close() }()
	defer func() { _ = w.Close() }()

	ui := &UI{out: w, mode: ModeTTY}
	uw := &uiWriter{ui: ui}
	ui.writer = uw

	if _, err := io.WriteString(ui.Writer(), "hello"); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := io.WriteString(ui.Writer(), " world\r\n"); err != nil {
		t.Fatalf("write: %v", err)
	}

	_ = w.Close()
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if want := "hello world\n"; string(got) != want {
		t.Fatalf("unexpected output:\nwant %q\ngot  %q", want, string(got))
	}
}

func TestUIClose_TTYFlushesBufferedLineToHistory(t *testing.T) {
	ui := &UI{
		mode:      ModeTTY,
		ttySendCh: make(chan tea.Msg, 1),
	}
	uw := &uiWriter{ui: ui}
	ui.writer = uw

	if _, err := io.WriteString(ui.Writer(), "hello"); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := ui.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	msg, ok := <-ui.ttySendCh
	if !ok {
		t.Fatalf("expected flushed history message")
	}
	body := teaPrintLineBody(t, msg)
	if want := "\rhello" + ansi.EraseLineRight; body != want {
		t.Fatalf("unexpected flushed msg body:\nwant %q\ngot  %q", want, body)
	}
}
