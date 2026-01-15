package progress

import (
	"bytes"
	"testing"

	tuiterm "github.com/pingcap/tiup/pkg/tui/term"
	"github.com/stretchr/testify/require"
)

type fakeTTYWriter struct {
	bytes.Buffer
}

func (*fakeTTYWriter) TUIMode() tuiterm.OutputMode {
	return tuiterm.OutputMode{Color: true, Control: true}
}

func TestUI_ModeAuto_NonFileWriter_ForcesPlain(t *testing.T) {
	out := &fakeTTYWriter{}
	ui := New(Options{Mode: ModeAuto, Out: out})
	require.Equal(t, ModePlain, ui.Mode())
	require.NoError(t, ui.Close())
}
