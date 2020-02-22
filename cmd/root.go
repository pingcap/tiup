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
	"github.com/spf13/cobra"
)

var rootCmd *cobra.Command

func init() {
	rootCmd = &cobra.Command{
		Use:   "tiup",
		Short: "Download and install TiDB components from command line",
		Long: `The tiup utility is a command line tool that can help to download
and installing TiDB components to the local system.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	rootCmd.AddCommand(
		newSelfCmd(),
		newComponentCmd(),
		newUpdateCmd(),
		newRunCmd(),
		newStatusCmd(),
		newLogCmd(),
		newStopCmd(),
		newShowCmd(),
		newVersionCmd(),
		newCompletionsCmd(),
	)
}

// Execute parses the command line argumnts and calls proper functions
func Execute() error {
	return rootCmd.Execute()
}
