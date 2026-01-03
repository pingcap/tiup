package main

// cliState holds process-level CLI state for both "tiup playground" (boot) and
// its subcommands (display/scale-in/scale-out).
//
// It intentionally avoids package-level mutable globals so tests and helpers can
// be pure and side-effect free.
type cliState struct {
	options BootOptions

	tag            string
	tiupDataDir    string
	dataDir        string
	deleteWhenExit bool
}

func newCLIState() *cliState {
	return &cliState{options: BootOptions{Monitor: true}}
}
