package progress

import (
	"io"
	"os"
	"testing"
)

func TestUIBlankLine_TTYWritesSingleNewlineWhenNoBufferedText(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer func() { _ = r.Close() }()
	defer func() { _ = w.Close() }()

	ui := &UI{out: w, mode: ModeTTY}
	uw := &uiWriter{ui: ui}
	ui.writer = uw

	ui.BlankLine()

	_ = w.Close()
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if string(got) != "\n" {
		t.Fatalf("unexpected output:\n%q", string(got))
	}
}

func TestUIBlankLine_TTYFlushesBufferedTextThenWritesBlankLine(t *testing.T) {
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
	ui.BlankLine()
	if _, err := io.WriteString(ui.Writer(), "next\n"); err != nil {
		t.Fatalf("write: %v", err)
	}

	_ = w.Close()
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	want := "hello\n\nnext\n"
	if string(got) != want {
		t.Fatalf("unexpected output:\nwant %q\ngot  %q", want, string(got))
	}
}

