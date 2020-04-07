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
	"time"

	"github.com/fatih/color"
)

var (
	spinnerText = []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")
)

var (
	colorDone    = color.New(color.FgHiGreen)
	colorError   = color.New(color.FgHiRed)
	colorSpinner = color.New(color.FgHiCyan)
)

const (
	refreshRate = time.Millisecond * 50
	doneTail    = "Done"
	errorTail   = "Error"
)

// Bar controls how a bar is displayed, for both single bar or multi bar item.
type Bar interface {
	UpdateDisplay(newDisplay *DisplayProps)
}
