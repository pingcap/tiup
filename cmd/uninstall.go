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
	"path/filepath"
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/spf13/cobra"
)

type uninstallOptions struct {
	all        bool
	self       bool
	mirrorList []string
}

func newUninstallCmd() *cobra.Command {
	var opt uninstallOptions
	cmdUnInst := &cobra.Command{
		Use:   "uninstall <component1>:<version>",
		Short: "Uninstall components or versions of a component",
		Long: `If you specify a version number, uninstall the specified version of
the component. You must use --all explicitly if you want to remove all
components or versions which are installed. You can uninstall multiple
components or multiple versions of a component at once. The --self flag
which is used to uninstall tiup.

  # Uninstall the specific version a component of default mirror
  tiup uninstall tidb:v3.0.10

  # Uninstall all versions of specific component of default mirror
  tiup uninstall tidb --all

  # Uninstall the specific version a component of specific mirror
  tiup uninstall tiup-mirrors.pingcap.com/tidb:v3.0.10

  # Uninstall all versions of a specific component of a specific mirror
  tiup uninstall tiup-mirrors.pingcap.com/tidb --all 

  # Uninstall all installed components of some specific mirrors
  tiup uninstall --all  --mirrors tiup-mirrors.pingcap.com

  # Uninstall all installed components of all mirrors
  tiup uninstall --all`,
		RunE: func(cmd *cobra.Command, args []string) error {
			teleCommand = cmd.CommandPath()
			if opt.self {
				deletable := []string{"bin", "manifest", "manifests", "components", "storage/cluster/packages"}
				for _, dir := range deletable {
					if err := os.RemoveAll(filepath.Join(tiupC.TiUPHomePath(), dir)); err != nil {
						return errors.Trace(err)
					}
					fmt.Printf("Remove directory '%s' successfully!\n", filepath.Join(tiupC.TiUPHomePath(), dir))
				}
				fmt.Printf("Uninstalled TiUP successfully! (User data reserved, you can delete '%s' manually if you confirm userdata useless)\n", tiupC.TiUPHomePath())
				return nil
			}

			if len(opt.mirrorList) == 0 {
				for _, m := range tiupC.ListMirrors() {
					opt.mirrorList = append(opt.mirrorList, m.Name)
				}

			}
			switch {
			case len(args) > 0 && len(tiupC.ListMirrors()) == 0:
				return removeComponents(environment.GlobalEnv(), args, opt.all)
			case len(args) > 0 && len(tiupC.ListMirrors()) > 0:
				for _, spec := range args {
					if !strings.Contains(spec, ":") && !opt.all {
						if !opt.all {
							fmt.Printf("Use `tiup uninstall %s --all` if you want to remove all versions.\n", spec)
							continue
						}
					}
					err := tiupC.Uninstall(spec)
					if err != nil {
						return errors.Trace(err)
					}
				}
				return nil
			case len(args) == 0 && len(opt.mirrorList) > 0:
				if !opt.all {
					fmt.Printf("Use `tiup uninstall --mirrors %s --all` if you want to remove all components.\n", strings.Join(opt.mirrorList, ","))
					return nil
				}
				for _, mirror := range opt.mirrorList {
					repo := tiupC.GetRepository(mirror)
					if err := os.RemoveAll(repo.Local().ProfilePath(localdata.ComponentParentDir, mirror)); err != nil {
						return errors.Trace(err)
					}
					fmt.Printf("Uninstalled all components of mirror %s successfully!\n", mirror)
				}
				return nil
			case len(args) == 0 && opt.all:
				if err := os.RemoveAll(environment.GlobalEnv().LocalPath(localdata.ComponentParentDir)); err != nil {
					return errors.Trace(err)
				}
				fmt.Println("Uninstalled all components successfully!")
				return nil
			default:
				return cmd.Help()
			}
		},
	}
	cmdUnInst.Flags().BoolVar(&opt.all, "all", false, "Remove all components or versions.")
	cmdUnInst.Flags().BoolVar(&opt.self, "self", false, "Uninstall tiup and clean all local data")
	cmdUnInst.Flags().StringSliceVar(&opt.mirrorList, "mirrors", []string{}, "Uninstall of some specify mirrors")
	return cmdUnInst
}

func removeComponents(env *environment.Environment, specs []string, all bool) error {
	for _, spec := range specs {
		paths := []string{}
		if strings.Contains(spec, ":") {
			parts := strings.SplitN(spec, ":", 2)
			// after this version is deleted, component will have no version left. delete the whole component dir directly
			dir, err := os.ReadDir(env.LocalPath(localdata.ComponentParentDir, parts[0]))
			if err != nil {
				return errors.Trace(err)
			}
			if parts[1] == utils.NightlyVersionAlias {
				for _, fi := range dir {
					if utils.Version(fi.Name()).IsNightly() {
						paths = append(paths, env.LocalPath(localdata.ComponentParentDir, parts[0], fi.Name()))
					}
				}
			} else {
				paths = append(paths, env.LocalPath(localdata.ComponentParentDir, parts[0], parts[1]))
			}
			if len(dir)-len(paths) < 1 {
				paths = append(paths, env.LocalPath(localdata.ComponentParentDir, parts[0]))
			}
		} else {
			if !all {
				fmt.Printf("Use `tiup uninstall %s --all` if you want to remove all versions.\n", spec)
				continue
			}
			paths = append(paths, env.LocalPath(localdata.ComponentParentDir, spec))
		}
		for _, path := range paths {
			if err := os.RemoveAll(path); err != nil {
				return errors.Trace(err)
			}
		}

		fmt.Printf("Uninstalled component `%s` successfully!\n", spec)
	}
	return nil
}
