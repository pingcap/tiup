package main

import (
	"fmt"
	"os"

	"github.com/pingcap/tiup/pkg/localdata"
)

const playgroundComponentName = "playground-ng"

func resolvePlaygroundCLIArg0(tiupVersion, userInputComponentVersion, argv0 string) string {
	// When running as a TiUP component, reconstruct the user-facing command line
	// prefix so help/examples are copy-pasteable:
	//   tiup playground-ng[:<component-version>]
	//
	// NOTE: TIUP_USER_INPUT_VERSION is only set when users specify the component
	// version explicitly (e.g. `tiup playground-ng:nightly ...`). It is empty for
	// `tiup playground-ng ...`.
	if tiupVersion != "" {
		if userInputComponentVersion != "" {
			return fmt.Sprintf("tiup %s:%s", playgroundComponentName, userInputComponentVersion)
		}
		return fmt.Sprintf("tiup %s", playgroundComponentName)
	}

	// Standalone mode: keep argv[0] as-is so it matches how users invoked the
	// binary (e.g. `bin/tiup-playground-ng`).
	if argv0 != "" {
		return argv0
	}
	return playgroundComponentName
}

func playgroundCLIArg0() string {
	argv0 := ""
	if len(os.Args) > 0 {
		argv0 = os.Args[0]
	}
	return resolvePlaygroundCLIArg0(
		os.Getenv(localdata.EnvNameTiUPVersion),
		os.Getenv(localdata.EnvNameUserInputVersion),
		argv0,
	)
}

func playgroundCLICommand(subcommand string) string {
	arg0 := playgroundCLIArg0()
	if subcommand == "" {
		return arg0
	}
	return arg0 + " " + subcommand
}
