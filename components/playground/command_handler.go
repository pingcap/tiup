package main

import (
	"fmt"
	"io"
)

func (p *Playground) handleCommand(state *controllerState, cmd *Command, w io.Writer) error {
	if cmd == nil {
		return fmt.Errorf("command is nil")
	}

	// Reject commands while stopping to keep lifecycle predictable.
	if p.Stopping() {
		return fmt.Errorf("playground is stopping")
	}

	switch cmd.Type {
	case DisplayCommandType:
		verbose := false
		jsonOut := false
		if cmd.Display != nil {
			verbose = cmd.Display.Verbose
			jsonOut = cmd.Display.JSON
		}
		return p.handleDisplay(state, w, verbose, jsonOut)
	case ScaleInCommandType:
		if cmd.ScaleIn == nil {
			return fmt.Errorf("missing scale_in request")
		}
		return p.handleScaleIn(state, w, cmd.ScaleIn)
	case ScaleOutCommandType:
		return p.handleScaleOut(state, w, cmd.ScaleOut)
	default:
		return fmt.Errorf("unknown command type: %s", cmd.Type)
	}
}
