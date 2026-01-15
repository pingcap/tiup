package progress

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUIWriter_BuffersUntilNewline(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })
	t.Cleanup(func() { _ = w.Close() })

	ui := New(Options{Mode: ModePlain, Out: w})

	_, err = io.WriteString(ui.Writer(), "hello")
	require.NoError(t, err)
	_, err = io.WriteString(ui.Writer(), " world\r\n")
	require.NoError(t, err)

	require.NoError(t, ui.Close())
	_ = w.Close()

	got, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, "hello world\n", string(got))
}

func TestUIWriter_EmitsPrintLinesBlockPerWrite(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })
	t.Cleanup(func() { _ = w.Close() })

	var eventBuf bytes.Buffer
	ui := New(Options{Mode: ModePlain, Out: w, EventLog: &eventBuf})

	_, err = io.WriteString(ui.Writer(), "a\nb\n")
	require.NoError(t, err)

	require.NoError(t, ui.Close())
	_ = w.Close()

	lines := bytes.Split(bytes.TrimSpace(eventBuf.Bytes()), []byte("\n"))
	require.Len(t, lines, 1)

	e, err := DecodeEvent(lines[0])
	require.NoError(t, err)
	require.Equal(t, EventPrintLines, e.Type)
	require.Equal(t, []string{"a", "b"}, e.Lines)
}

func TestUIClose_FlushesBufferedLine(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })
	t.Cleanup(func() { _ = w.Close() })

	ui := New(Options{Mode: ModePlain, Out: w})

	_, err = io.WriteString(ui.Writer(), "hello")
	require.NoError(t, err)

	require.NoError(t, ui.Close())
	_ = w.Close()

	got, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, "hello\n", string(got))
}

func TestUIPrintLines_FlushesBufferedTextThenWritesBlankLine(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })
	t.Cleanup(func() { _ = w.Close() })

	ui := New(Options{Mode: ModePlain, Out: w})

	_, err = io.WriteString(ui.Writer(), "hello")
	require.NoError(t, err)
	ui.PrintLines([]string{""})
	_, err = io.WriteString(ui.Writer(), "next\n")
	require.NoError(t, err)

	require.NoError(t, ui.Close())
	_ = w.Close()

	got, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, "hello\n\nnext\n", string(got))
}
