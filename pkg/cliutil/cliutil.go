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
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/joomcode/errorx"
	"github.com/pingcap-incubator/tiup-cluster/pkg/colorutil"
	"github.com/pingcap-incubator/tiup-cluster/pkg/errutil"
	"github.com/pingcap-incubator/tiup/pkg/localdata"
	"github.com/spf13/cobra"
)

var (
	errNS             = errorx.NewNamespace("cliutil")
	errMismatchArgs   = errNS.NewType("mismatch_args", errutil.ErrTraitPreCheck)
	errOperationAbort = errNS.NewType("operation_aborted", errutil.ErrTraitPreCheck)
)

var templateFuncs = template.FuncMap{
	"OsArgs":  OsArgs,
	"OsArgs0": OsArgs0,
}

func args() []string {
	if wd := os.Getenv(localdata.EnvNameWorkDir); wd != "" {
		// FIXME: We should use TiUp's arg0 instead of hardcode
		return append([]string{"tiup cluster"}, os.Args[1:]...)
	}
	return os.Args
}

// OsArgs return the whole command line that user inputs, e.g. tiops deploy --xxx, or tiup cluster deploy --xxx
func OsArgs() string {
	return strings.Join(args(), " ")
}

// OsArgs0 return the command name that user inputs, e.g. tiops, or tiup cluster.
func OsArgs0() string {
	return args()[0]
}

func init() {
	colorutil.AddColorFunctions(func(name string, f interface{}) {
		templateFuncs[name] = f
	})
}

// CheckCommandArgsAndMayPrintHelp checks whether user passes enough number of arguments.
// If insufficient number of arguments are passed, an error with proper suggestion will be raised.
// When no argument is passed, command help will be printed and no error will be raised.
func CheckCommandArgsAndMayPrintHelp(cmd *cobra.Command, args []string, minArgs int) (shouldContinue bool, err error) {
	if minArgs == 0 {
		return true, nil
	}
	lenArgs := len(args)
	if lenArgs == 0 {
		return false, cmd.Help()
	}
	if lenArgs < minArgs {
		return false, errMismatchArgs.
			New("Expect at least %d arguments, but received %d arguments", minArgs, lenArgs).
			WithProperty(SuggestionFromString(cmd.UsageString()))
	}
	return true, nil
}

func formatSuggestion(templateStr string, data interface{}) string {
	t := template.Must(template.New("suggestion").Funcs(templateFuncs).Parse(templateStr))
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		panic(err)
	}
	return buf.String()
}

// SuggestionFromString creates a suggestion from string.
// Usage: SomeErrorX.WithProperty(SuggestionFromString(..))
func SuggestionFromString(str string) (errorx.Property, string) {
	return errutil.ErrPropSuggestion, strings.TrimSpace(str)
}

// SuggestionFromTemplate creates a suggestion from go template. Colorize function and some other utilities
// are available.
// Usage: SomeErrorX.WithProperty(SuggestionFromTemplate(..))
func SuggestionFromTemplate(templateStr string, data interface{}) (errorx.Property, string) {
	return SuggestionFromString(formatSuggestion(templateStr, data))
}

// SuggestionFromFormat creates a suggestion from a format.
// Usage: SomeErrorX.WithProperty(SuggestionFromFormat(..))
func SuggestionFromFormat(format string, a ...interface{}) (errorx.Property, string) {
	s := fmt.Sprintf(format, a...)
	return SuggestionFromString(s)
}
