package progress

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUIBlankLine_TTYWritesSingleNewlineWhenNoBufferedText(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })
	t.Cleanup(func() { _ = w.Close() })

	ui := &UI{out: w, mode: ModeTTY}
	uw := &uiWriter{ui: ui}
	ui.writer = uw

	ui.BlankLine()

	_ = w.Close()
	got, err := io.ReadAll(r)
	require.NoError(t, err)

	require.Equal(t, "\n", string(got))
}

func TestUIBlankLine_TTYFlushesBufferedTextThenWritesBlankLine(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })
	t.Cleanup(func() { _ = w.Close() })

	ui := &UI{out: w, mode: ModeTTY}
	uw := &uiWriter{ui: ui}
	ui.writer = uw

	_, err = io.WriteString(ui.Writer(), "hello")
	require.NoError(t, err)
	ui.BlankLine()
	_, err = io.WriteString(ui.Writer(), "next\n")
	require.NoError(t, err)

	_ = w.Close()
	got, err := io.ReadAll(r)
	require.NoError(t, err)

	want := "hello\n\nnext\n"
	require.Equal(t, want, string(got))
}
