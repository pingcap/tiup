// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package progress

import (
	"encoding/json"
	"strings"
)

// Mode determines how the progress bar is rendered
type Mode int

const (
	// ModeSpinner renders a Spinner
	ModeSpinner Mode = iota
	// ModeProgress renders a ProgressBar. Not supported yet.
	ModeProgress
	// ModeDone renders as "Done" message.
	ModeDone
	// ModeError renders as "Error" message.
	ModeError
)

// MarshalJSON implements JSON marshaler
func (m Mode) MarshalJSON() ([]byte, error) {
	var s string
	switch m {
	case ModeSpinner:
		s = "spinner"
	case ModeProgress:
		s = "progress"
	case ModeDone:
		s = "done"
	case ModeError:
		s = "error"
	default:
		s = "unknown"
	}
	return json.Marshal(s)
}

// UnmarshalJSON implements JSON unmarshaler
func (m *Mode) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	switch strings.ToLower(s) {
	case "spinner":
		*m = ModeSpinner
	case "progress":
		*m = ModeProgress
	case "done":
		*m = ModeDone
	case "error":
		*m = ModeError
	default:
		panic("unknown mode")
	}

	return nil
}

// DisplayProps controls the display of the progress bar.
type DisplayProps struct {
	Prefix string `json:"prefix,omitempty"`
	Suffix string `json:"suffix,omitempty"` // If `Mode == Done / Error`, Suffix is not printed
	Mode   Mode   `json:"mode,omitempty"`
}
