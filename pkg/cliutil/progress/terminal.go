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
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/atomic"
	"golang.org/x/sys/unix"
)

var (
	termSizeWidth  = atomic.Int32{}
	termSizeHeight = atomic.Int32{}
)

func updateTerminalSize() error {
	ws, err := unix.IoctlGetWinsize(syscall.Stdout, unix.TIOCGWINSZ)
	if err != nil {
		return err
	}
	termSizeWidth.Store(int32(ws.Col))
	termSizeHeight.Store(int32(ws.Row))
	return nil
}

func moveCursorUp(w io.Writer, n int) {
	_, _ = fmt.Fprintf(w, "\033[%dA", n)
}

func moveCursorDown(w io.Writer, n int) {
	_, _ = fmt.Fprintf(w, "\033[%dB", n)
}

func moveCursorToLineStart(w io.Writer) {
	_, _ = fmt.Fprintf(w, "\r")
}

func clearLine(w io.Writer) {
	_, _ = fmt.Fprintf(w, "\033[2K")
}

func init() {
	_ = updateTerminalSize()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)

	go func() {
		for {
			if _, ok := <-sigCh; !ok {
				return
			}
			_ = updateTerminalSize()
		}
	}()
}
