package tui

import (
	"os"
	"testing"

	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/stretchr/testify/require"
)

func TestOsArgs0_StandaloneKeepsPathPrefix(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })

	t.Cleanup(func() {
		_ = os.Unsetenv(localdata.EnvNameTiUPVersion)
	})

	require.NoError(t, os.Unsetenv(localdata.EnvNameTiUPVersion))

	os.Args = []string{"tiup-playground-ng"}
	require.Equal(t, "tiup-playground-ng", OsArgs0())

	os.Args = []string{"bin/tiup-playground-ng"}
	require.Equal(t, "bin/tiup-playground-ng", OsArgs0())
}

func TestOsArgs0_TiupModeUsesRegisteredArg0(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })

	t.Cleanup(func() {
		_ = os.Unsetenv(localdata.EnvNameTiUPVersion)
		RegisterArg0("tiup cluster")
	})

	require.NoError(t, os.Setenv(localdata.EnvNameTiUPVersion, "v9.9.9"))
	RegisterArg0("tiup playground-ng:nightly")

	os.Args = []string{"/tmp/tiup-playground-ng", "--help"}
	require.Equal(t, "tiup playground-ng:nightly", OsArgs0())
}
