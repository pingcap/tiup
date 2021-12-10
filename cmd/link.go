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
	"fmt"
	"os"
	"path/filepath"

	"github.com/pingcap/tiup/pkg/environment"
	"github.com/spf13/cobra"
)

func newLinkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "link <component>[:version]",
		Short: "Link component binary to $PATH",
		Long: `[experimental feature]
Link component binary to $PATH`,
		RunE: func(cmd *cobra.Command, args []string) error {
			teleCommand = cmd.CommandPath()
			env := environment.GlobalEnv()
			if len(args) != 1 {
				return cmd.Help()
			}
			component, version := environment.ParseCompVersion(args[0])
			if version == "" {
				var err error
				version, err = env.SelectInstalledVersion(component, version)
				if err != nil {
					return err
				}
			}
			binPath, _ := env.BinaryPath(component, version)
			target := filepath.Join(env.LocalPath("bin"), filepath.Base(binPath))
			fmt.Printf("package %s provides these executables: %s\n", component, filepath.Base(binPath))
			_ = os.Remove(target)
			return os.Symlink(binPath, target)
		},
	}
	return cmd
}
