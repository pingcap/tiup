package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolvePlaygroundCLIArg0(t *testing.T) {
	t.Run("StandaloneUsesArgv0", func(t *testing.T) {
		got := resolvePlaygroundCLIArg0("", "", "bin/playground-ng")
		require.Equal(t, "bin/playground-ng", got)
	})

	t.Run("StandaloneFallsBackToComponentName", func(t *testing.T) {
		got := resolvePlaygroundCLIArg0("", "", "")
		require.Equal(t, playgroundComponentName, got)
	})

	t.Run("TiUPImplicitComponentVersionOmitsColon", func(t *testing.T) {
		got := resolvePlaygroundCLIArg0("v1.0.0", "", "/tmp/ignored")
		require.Equal(t, "tiup playground-ng", got)
	})

	t.Run("TiUPExplicitComponentVersionIncludesColon", func(t *testing.T) {
		got := resolvePlaygroundCLIArg0("v1.0.0", "nightly", "/tmp/ignored")
		require.Equal(t, "tiup playground-ng:nightly", got)
	})
}
