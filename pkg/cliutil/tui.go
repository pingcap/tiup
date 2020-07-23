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

package cliutil

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/AstroProfundis/tabby"
	"golang.org/x/crypto/ssh/terminal"
)

// PrintTable accepts a matrix of strings and print them as ASCII table to terminal
func PrintTable(rows [][]string, header bool) {
	// Print the table
	t := tabby.New()
	if header {
		addRow(t, rows[0], header)
		rows = rows[1:]
	}
	for _, row := range rows {
		addRow(t, row, false)
	}
	t.Print()
}

func addRow(t *tabby.Tabby, rawLine []string, header bool) {
	// Convert []string to []interface{}
	row := make([]interface{}, len(rawLine))
	for i, v := range rawLine {
		row[i] = v
	}

	// Add line to the table
	if header {
		t.AddHeader(row...)
	} else {
		t.AddLine(row...)
	}
}

// Prompt accepts input from console by user
func Prompt(prompt string) string {
	if prompt != "" {
		prompt += " " // append a whitespace
	}
	fmt.Print(prompt)

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return ""
	}
	return strings.TrimSuffix(input, "\n")
}

// PromptForConfirm accepts yes / no from console by user, default to No
func PromptForConfirm(format string, a ...interface{}) bool {
	ans := Prompt(fmt.Sprintf(format, a...))
	switch strings.TrimSpace(strings.ToLower(ans)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}

// PromptForConfirmReverse accepts yes / no from console by user, default to Yes
func PromptForConfirmReverse(format string, a ...interface{}) bool {
	return !PromptForConfirm(format, a...)
}

// PromptForConfirmOrAbortError accepts yes / no from console by user, generates AbortError if user does not input yes.
func PromptForConfirmOrAbortError(format string, a ...interface{}) error {
	if !PromptForConfirm(format, a...) {
		return errOperationAbort.New("Operation aborted by user")
	}
	return nil
}

// PromptForPassword reads a password input from console
func PromptForPassword(format string, a ...interface{}) string {
	defer fmt.Println("")

	fmt.Printf(format, a...)

	input, err := terminal.ReadPassword(syscall.Stdin)

	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.Trim(string(input), "\n"))
}

// OsArch builds an "os/arch" string from input, it converts some similar strings
// to different words to avoid misreading when displaying in terminal
func OsArch(os, arch string) string {
	osFmt := os
	archFmt := arch

	switch arch {
	case "amd64":
		archFmt = "x86_64"
	case "arm64":
		archFmt = "aarch64"
	}

	return fmt.Sprintf("%s/%s", osFmt, archFmt)
}
