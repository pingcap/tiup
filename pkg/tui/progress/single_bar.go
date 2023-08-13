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
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/fatih/color"
	"github.com/mattn/go-runewidth"
	"go.uber.org/atomic"
)

type singleBarCore struct {
	displayProps atomic.Value
	spinnerFrame int
}

func (b *singleBarCore) renderDoneOrError(w io.Writer, dp *DisplayProps) {
	width := int(termSizeWidth.Load())
	var tail, detail string
	var tailColor *color.Color
	switch dp.Mode {
	case ModeDone:
		tail = doneTail
		tailColor = colorDone
	case ModeError:
		tail = errorTail
		tailColor = colorError
	default:
		panic("Unexpect dp.Mode")
	}
	var displayPrefix string
	midWidth := 1 + 3 + 1 + len(tail)
	prefixWidth := runewidth.StringWidth(dp.Prefix)
	if midWidth+prefixWidth <= width || midWidth > width {
		displayPrefix = dp.Prefix
	} else {
		displayPrefix = runewidth.Truncate(dp.Prefix, width-prefixWidth, "")
	}
	if len(dp.Detail) > 0 {
		detail = ": " + dp.Detail
	}
	_, _ = fmt.Fprintf(w, "%s ... %s%s", displayPrefix, tailColor.Sprint(tail), detail)
}

func (b *singleBarCore) renderSpinner(w io.Writer, dp *DisplayProps) {
	width := int(termSizeWidth.Load())

	var displayPrefix, displaySuffix string
	midWidth := 1 + 3 + 1 + 1 + 1
	prefixWidth := runewidth.StringWidth(dp.Prefix)
	suffixWidth := runewidth.StringWidth(dp.Suffix)
	switch {
	case midWidth+prefixWidth+suffixWidth <= width || midWidth > width:
		// If screen is too small, do not fit it any more.
		displayPrefix = dp.Prefix
		displaySuffix = dp.Suffix
	case midWidth+prefixWidth <= width:
		displayPrefix = dp.Prefix
		displaySuffix = runewidth.Truncate(dp.Suffix, width-midWidth-prefixWidth, "...")
	default:
		displayPrefix = runewidth.Truncate(dp.Prefix, width-midWidth, "")
		displaySuffix = ""
	}
	_, _ = fmt.Fprintf(w, "%s ... %s %s",
		displayPrefix,
		colorSpinner.Sprintf("%c", spinnerText[b.spinnerFrame]),
		displaySuffix)

	b.spinnerFrame = (b.spinnerFrame + 1) % len(spinnerText)
}

func (b *singleBarCore) renderTo(w io.Writer) {
	dp := (b.displayProps.Load()).(*DisplayProps)
	if dp.Mode == ModeDone || dp.Mode == ModeError {
		b.renderDoneOrError(w, dp)
	} else {
		b.renderSpinner(w, dp)
	}
}

func newSingleBarCore(prefix string) singleBarCore {
	c := singleBarCore{
		displayProps: atomic.Value{},
		spinnerFrame: 0,
	}
	c.displayProps.Store(&DisplayProps{
		Prefix: prefix,
		Mode:   ModeSpinner,
	})
	return c
}

// SingleBar renders single progress bar.
type SingleBar struct {
	core     singleBarCore
	renderer *renderer
}

// NewSingleBar creates a new SingleBar.
func NewSingleBar(prefix string) *SingleBar {
	b := &SingleBar{
		core:     newSingleBarCore(prefix),
		renderer: newRenderer(),
	}
	b.renderer.renderFn = b.render
	return b
}

// UpdateDisplay updates the display property of this single bar.
// This function is thread safe.
func (b *SingleBar) UpdateDisplay(newDisplay *DisplayProps) {
	b.core.displayProps.Store(newDisplay)
}

// StartRenderLoop starts the render loop.
// This function is thread safe.
func (b *SingleBar) StartRenderLoop() {
	b.preRender()
	b.renderer.startRenderLoop()
}

// StopRenderLoop stops the render loop.
// This function is thread safe.
func (b *SingleBar) StopRenderLoop() {
	b.renderer.stopRenderLoop()
}

func (b *SingleBar) preRender() {
	// Preserve space for the bar
	fmt.Println("")
}

func (b *SingleBar) render() {
	f := bufio.NewWriter(os.Stdout)

	moveCursorUp(f, 1)
	moveCursorToLineStart(f)
	clearLine(f)

	b.core.renderTo(f)

	moveCursorDown(f, 1)
	moveCursorToLineStart(f)
	_ = f.Flush()
}
