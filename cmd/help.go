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

package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

func newHelpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "help [command]",
		Short: "Help about any command or component",
		Long: `Help provides help for any command or component in the application.
Simply type tiup help <command>|<component> for full details.`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd, n, e := cmd.Root().Find(args)
			if (cmd == rootCmd || e != nil) && len(n) > 0 {
				externalHelp(n[0])
			} else {
				cmd.InitDefaultHelpFlag() // make possible 'help' flag to be shown
				cmd.HelpFunc()(cmd, args)
			}
		},
	}
}

func externalHelp(spec string) {
	binaryPath, err := binaryPath(spec)
	if err != nil {
		fmt.Println(err)
		return
	}
	comp := exec.Command(binaryPath, "-h")
	comp.Stdout = os.Stdout
	comp.Stderr = os.Stderr
	if err := comp.Start(); err != nil {
		fmt.Printf("Cannot fetch help message from %s failed: %v\n", binaryPath, err)
		return
	}
	if err := comp.Wait(); err != nil {
		fmt.Printf("Cannot fetch help message from %s failed: %v\n", binaryPath, err)
	}
}

func usageTemplate() string {
	var installComps string
	if repo := profile.Manifest(); repo != nil && len(repo.Components) > 0 {
		installComps = `
Available Components:
`
		var maxNameLen int
		for _, comp := range repo.Components {
			if len(comp.Name) > maxNameLen {
				maxNameLen = len(comp.Name)
			}
		}

		for _, comp := range repo.Components {
			installComps = installComps + fmt.Sprintf("  %s%s   %s\n", comp.Name, strings.Repeat(" ", maxNameLen-len(comp.Name)), comp.Desc)
		}
	} else {
		installComps = `
Components Manifest:
  use "tiup list --refresh" to fetch the latest components manifest
`
	}

	return `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}
` + installComps + `
Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`
}
