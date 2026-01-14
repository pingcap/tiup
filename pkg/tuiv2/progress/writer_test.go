package progress

import (
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

func TestUIBlankLine_FlushesBufferedTextThenWritesBlankLine(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })
	t.Cleanup(func() { _ = w.Close() })

	ui := New(Options{Mode: ModePlain, Out: w})

	_, err = io.WriteString(ui.Writer(), "hello")
	require.NoError(t, err)
	ui.BlankLine()
	_, err = io.WriteString(ui.Writer(), "next\n")
	require.NoError(t, err)

	require.NoError(t, ui.Close())
	_ = w.Close()

	got, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, "hello\n\nnext\n", string(got))
}
