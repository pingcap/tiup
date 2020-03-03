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
	"sort"

	"github.com/c4pt0r/tiup/pkg/meta"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

func newBinaryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "binary <component>:[version]",
		Short: "Print the binary path of a specific version of a component",
		Long:`Print the binary path of a specific version of a component, and the
latest version installed will be selected if no version specified.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}
			component, version := meta.ParseCompVersion(args[0])
			installed, err := profile.InstalledVersions(component)
			if err != nil {
				return err
			}

			errInstallFirst := fmt.Errorf("use `tiup install %[1]s` to install `%[1]s` first", args[0])
			if len(installed) < 1 {
				return errInstallFirst
			}
			if version.IsEmpty() {
				sort.Slice(installed, func(i, j int) bool {
					return semver.Compare(installed[i], installed[j]) < 0
				})
				version = meta.Version(installed[len(installed)-1])
			}
			found := false
			for _, v := range installed {
				if meta.Version(v) == version {
					found = true
					break
				}
			}
			if !found {
				return errInstallFirst
			}
			binaryPath, err := profile.BinaryPath(component, version)
			if err != nil {
				return err
			}
			fmt.Println(binaryPath)
			return nil
		},
	}
	return cmd
}
