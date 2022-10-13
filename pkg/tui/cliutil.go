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

package tui

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/joomcode/errorx"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/spf13/cobra"
)

var (
	errNS             = errorx.NewNamespace("tui")
	errMismatchArgs   = errNS.NewType("mismatch_args", utils.ErrTraitPreCheck)
	errOperationAbort = errNS.NewType("operation_aborted", utils.ErrTraitPreCheck)
)

var templateFuncs = template.FuncMap{
	"OsArgs":  OsArgs,
	"OsArgs0": OsArgs0,
}

// FIXME: We should use TiUP's arg0 instead of hardcode
var arg0 = "tiup cluster"

// RegisterArg0 register arg0
func RegisterArg0(s string) {
	arg0 = s
}

func args() []string {
	// if running in TiUP component mode
	if wd := os.Getenv(localdata.EnvNameTiUPVersion); wd != "" {
		return append([]string{arg0}, os.Args[1:]...)
	}
	return os.Args
}

// OsArgs return the whole command line that user inputs, e.g. tiup deploy --xxx, or tiup cluster deploy --xxx
func OsArgs() string {
	return strings.Join(args(), " ")
}

// OsArgs0 return the command name that user inputs, e.g. tiup, or tiup cluster.
func OsArgs0() string {
	if strings.Contains(args()[0], " ") {
		return args()[0]
	}
	return filepath.Base(args()[0])
}

func init() {
	AddColorFunctions(func(name string, f any) {
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

func formatSuggestion(templateStr string, data any) string {
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
	return utils.ErrPropSuggestion, strings.TrimSpace(str)
}

// SuggestionFromTemplate creates a suggestion from go template. Colorize function and some other utilities
// are available.
// Usage: SomeErrorX.WithProperty(SuggestionFromTemplate(..))
func SuggestionFromTemplate(templateStr string, data any) (errorx.Property, string) {
	return SuggestionFromString(formatSuggestion(templateStr, data))
}

// SuggestionFromFormat creates a suggestion from a format.
// Usage: SomeErrorX.WithProperty(SuggestionFromFormat(..))
func SuggestionFromFormat(format string, a ...any) (errorx.Property, string) {
	s := fmt.Sprintf(format, a...)
	return SuggestionFromString(s)
}

// BeautifyCobraUsageAndHelp beautifies cobra usages and help.
func BeautifyCobraUsageAndHelp(rootCmd *cobra.Command) {
	s := `Usage:{{if .Runnable}}
  {{ColorCommand}}{{tiupCmdLine .UseLine}}{{ColorReset}}{{end}}{{if .HasAvailableSubCommands}}
  {{ColorCommand}}{{tiupCmdPath .Use}} [command]{{ColorReset}}{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{ColorCommand}}{{.NameAndAliases}}{{ColorReset}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{ColorCommand}}{{tiupCmdPath .Use}} help [command]{{ColorReset}}" for more information about a command.{{end}}
`
	cobra.AddTemplateFunc("tiupCmdLine", cmdLine)
	cobra.AddTemplateFunc("tiupCmdPath", cmdPath)

	rootCmd.SetUsageTemplate(s)
}

// cmdLine is a customized cobra.Command.UseLine()
func cmdLine(useline string) string {
	i := strings.Index(useline, " ")
	if i > 0 {
		return OsArgs0() + useline[i:]
	}
	return useline
}

// cmdPath is a customized cobra.Command.CommandPath()
func cmdPath(use string) string {
	if strings.Contains(use, " ") {
		use = OsArgs0()
	}
	return use
}
