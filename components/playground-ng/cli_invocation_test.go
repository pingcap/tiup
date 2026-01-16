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

func TestRewriteCobraUseLine(t *testing.T) {
	arg0 := "tiup playground-ng:v1.2.3"

	t.Run("RewritesRootUseLine", func(t *testing.T) {
		got := rewriteCobraUseLine(arg0, "tiup-playground-ng [version]")
		require.Equal(t, "tiup playground-ng:v1.2.3 [version]", got)
	})

	t.Run("RewritesSubcommandUseLine", func(t *testing.T) {
		got := rewriteCobraUseLine(arg0, "tiup-playground-ng stop [flags]")
		require.Equal(t, "tiup playground-ng:v1.2.3 stop [flags]", got)
	})

	t.Run("NoSuffix", func(t *testing.T) {
		got := rewriteCobraUseLine(arg0, "tiup-playground-ng")
		require.Equal(t, arg0, got)
	})
}

func TestRewriteCobraCommandPath(t *testing.T) {
	arg0 := "tiup playground-ng:v1.2.3"

	t.Run("RewritesRootCommandPath", func(t *testing.T) {
		got := rewriteCobraCommandPath(arg0, "tiup-playground-ng")
		require.Equal(t, arg0, got)
	})

	t.Run("RewritesSubcommandCommandPath", func(t *testing.T) {
		got := rewriteCobraCommandPath(arg0, "tiup-playground-ng help")
		require.Equal(t, "tiup playground-ng:v1.2.3 help", got)
	})
}
