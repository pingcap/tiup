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
	"io/ioutil"
	"os"
	"strings"

	"github.com/pingcap-incubator/tiup/pkg/localdata"
	"github.com/pingcap-incubator/tiup/pkg/meta"
	"github.com/pingcap/errors"
	"github.com/spf13/cobra"
)

func newUninstallCmd(env *meta.Environment) *cobra.Command {
	var all, self bool
	cmdUnInst := &cobra.Command{
		Use:   "uninstall <component1>:<version>",
		Short: "Uninstall components or versions of a component",
		Long: `If you specify a version number, uninstall the specified version of
the component. You must use --all explicitly if you want to remove all
components or versions which are installed. You can uninstall multiple
components or multiple versions of a component at once. The --self flag
which is used to uninstall tiup.

  # Uninstall tiup
  tiup uninstall --self

  # Uninstall the specific version a component
  tiup uninstall tidb:v3.0.10

  # Uninstall all version of specific component
  tiup uninstall tidb --all

  # Uninstall all installed components
  tiup uninstall --all`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if self {
				deletable := []string{"bin", "manifest", "manifests", "components", "storage/cluster/packages"}
				for _, dir := range deletable {
					if err := os.RemoveAll(env.Profile().Path(dir)); err != nil {
						return errors.Trace(err)
					}
					fmt.Printf("Remove directory '%s' successfully!\n", env.Profile().Path(dir))
				}
				fmt.Printf("Uninstalled TiUP successfully! (User data reserved, you can delete '%s' manually if you confirm userdata useless)\n", env.Profile().Root())
				return nil
			}
			switch {
			case len(args) > 0:
				return removeComponents(env, args, all)
			case len(args) == 0 && all:
				if err := os.RemoveAll(env.LocalPath(localdata.ComponentParentDir)); err != nil {
					return errors.Trace(err)
				}
				fmt.Println("Uninstalled all components successfully!")
				return nil
			default:
				return cmd.Help()
			}
		},
	}
	cmdUnInst.Flags().BoolVar(&all, "all", false, "Remove all components or versions.")
	cmdUnInst.Flags().BoolVar(&self, "self", false, "Uninstall tiup and clean all local data")
	return cmdUnInst
}

func removeComponents(env *meta.Environment, specs []string, all bool) error {
	for _, spec := range specs {
		var path string
		if strings.Contains(spec, ":") {
			parts := strings.SplitN(spec, ":", 2)
			// after this version is deleted, component will have no version left. delete the whole component dir directly
			if dir, err := ioutil.ReadDir(env.LocalPath(localdata.ComponentParentDir, parts[0])); err == nil && len(dir) <= 1 {
				path = env.LocalPath(localdata.ComponentParentDir, parts[0])
			} else {
				path = env.LocalPath(localdata.ComponentParentDir, parts[0], parts[1])
			}
		} else {
			if !all {
				fmt.Printf("Use `tiup uninstall %s --all` if you want to remove all versions.\n", spec)
				continue
			}
			if err := os.RemoveAll(env.LocalPath(localdata.StorageParentDir, spec)); err != nil {
				return err
			}
			path = env.LocalPath(localdata.ComponentParentDir, spec)
		}
		err := os.RemoveAll(path)
		if err != nil {
			return err
		}
		fmt.Printf("Uninstalled component `%s` successfully!\n", spec)
	}
	return nil
}
