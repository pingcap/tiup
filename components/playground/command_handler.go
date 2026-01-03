package main

import (
	"fmt"
	"io"
)

func (p *Playground) handleCommand(cmd *Command, w io.Writer) error {
	if cmd == nil {
		return fmt.Errorf("command is nil")
	}

	// Reject state-changing commands while stopping to keep lifecycle predictable.
	if cmd.Type != DisplayCommandType {
		if p.Stopping() {
			return fmt.Errorf("playground is stopping")
		}
	}

	switch cmd.Type {
	case DisplayCommandType:
		verbose := false
		jsonOut := false
		if cmd.Display != nil {
			verbose = cmd.Display.Verbose
			jsonOut = cmd.Display.JSON
		}
		return p.handleDisplay(w, verbose, jsonOut)
	case ScaleInCommandType:
		if cmd.ScaleIn == nil {
			return fmt.Errorf("missing scale_in request")
		}
		return p.handleScaleIn(w, cmd.ScaleIn)
	case ScaleOutCommandType:
		return p.handleScaleOut(w, cmd.ScaleOut)
	default:
		return fmt.Errorf("unknown command type: %s", cmd.Type)
	}
}
