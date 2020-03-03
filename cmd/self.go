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
	"os"

	"github.com/spf13/cobra"
)

func newSelfCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "self",
		Short: "Modify the tiup installation",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "update",
			Short: "Update tiup to the latest version",
			RunE: func(cmd *cobra.Command, args []string) error {
				return repository.DownloadFile(profile.Path("bin"), "tiup")
			},
		},
		&cobra.Command{
			Use:   "uninstall",
			Short: "Uninstall tiup and clean all downloaded data",
			RunE: func(cmd *cobra.Command, args []string) error {
				return os.RemoveAll(profile.Root())
			},
		},
	)
	return cmd
}
