// Copyright 2021 PingCAP, Inc.
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
	"path/filepath"

	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/tui"
	"github.com/spf13/cobra"
)

func newUnlinkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unlink <component>",
		Short: "Unlink component binary to $TIUP_HOME/bin/",
		Long:  `[experimental] Unlink component binary in $TIUP_HOME/bin/`,
		RunE: func(cmd *cobra.Command, args []string) error {
			teleCommand = cmd.CommandPath()
			env := environment.GlobalEnv()
			if len(args) != 1 {
				return cmd.Help()
			}
			component, version := environment.ParseCompVersion(args[0])
			version, err := env.SelectInstalledVersion(component, version)
			if err != nil {
				return err
			}
			binPath, err := env.BinaryPath(component, version)
			if err != nil {
				return err
			}
			target := env.LocalPath("bin", filepath.Base(binPath))
			if err := tui.PromptForConfirmOrAbortError("%s will be removed.\n Do you want to continue? [y/N]:", target); err != nil {
				return err
			}
			return os.Remove(target)
		},
	}
	return cmd
}
