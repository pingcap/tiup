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
	"sort"
	"strings"

	"github.com/c4pt0r/tiup/pkg/localdata"
	"github.com/c4pt0r/tiup/pkg/meta"
	"github.com/c4pt0r/tiup/pkg/set"
	"github.com/c4pt0r/tiup/pkg/tui"
	"github.com/pingcap/errors"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

var defaultMirror = "https://tiup-mirrors.pingcap.com/"

func newComponentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "component",
		Short:   "Manage TiDB related components",
		Aliases: []string{"comp"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		newListComponentCmd(),
		newAddCmd(),
		newRemoveCmd(),
		newBinaryCmd(),
	)
	return cmd
}

func newListComponentCmd() *cobra.Command {
	var (
		showInstalled bool
		refresh       bool
	)
	cmdListComponent := &cobra.Command{
		Use:   "list",
		Short: "List the available TiDB components",
		RunE: func(cmd *cobra.Command, args []string) error {
			switch len(args) {
			case 0:
				if refresh || profile.Manifest() == nil {
					manifest, err := repository.Manifest()
					if err != nil {
						return err
					}
					err = profile.SaveManifest(manifest)
					if err != nil {
						return err
					}
				}
				return showComponentList(showInstalled)
			case 1:
				component := args[0]
				if refresh || profile.Versions(component) == nil {
					manifest, err := repository.ComponentVersions(component)
					if err != nil {
						return errors.Trace(err)
					}
					err = profile.SaveVersions(component, manifest)
					if err != nil {
						return err
					}
				}
				return showComponentVersions(component, showInstalled)
			default:
				return cmd.Help()
			}
		},
	}

	cmdListComponent.Flags().BoolVar(&showInstalled, "installed", false, "List installed components only.")
	cmdListComponent.Flags().BoolVar(&refresh, "refresh", false, "Refresh local components list cache.")
	return cmdListComponent
}

func showComponentList(onlyInstalled bool) error {
	installed, err := profile.InstalledComponents()
	if err != nil {
		return err
	}
	manifest := profile.Manifest()
	var cmpTable [][]string
	cmpTable = append(cmpTable, []string{"Name", "Installed", "Platforms", "Description"})

	localComponents := set.NewStringSet(installed...)
	for _, comp := range manifest.Components {
		if onlyInstalled && !localComponents.Exist(comp.Name) {
			continue
		}
		installStatus := ""
		if localComponents.Exist(comp.Name) {
			installStatus = "YES"
		}
		cmpTable = append(cmpTable, []string{
			comp.Name,
			installStatus,
			strings.Join(comp.Platforms, ","),
			comp.Desc,
		})
	}

	fmt.Printf("Available components (Last Modified: %s):\n", manifest.Modified)
	tui.PrintTable(cmpTable, true)
	return nil
}

func showComponentVersions(component string, onlyInstalled bool) error {
	versions, err := profile.InstalledVersions(component)
	if err != nil {
		return err
	}
	manifest := profile.Versions(component)

	var cmpTable [][]string
	cmpTable = append(cmpTable, []string{"Version", "Installed", "Release:", "Platforms"})

	installed := set.NewStringSet(versions...)
	for _, ver := range manifest.Versions {
		version := ver.Version.String()
		if onlyInstalled && !installed.Exist(version) {
			continue
		}
		installStatus := ""
		if installed.Exist(version) {
			installStatus = "YES"
		}
		cmpTable = append(cmpTable, []string{
			version,
			installStatus,
			ver.Date,
			strings.Join(ver.Platforms, ","),
		})
	}

	fmt.Printf("Available versions for %s (Last Modified: %s):\n", component, manifest.Modified)
	tui.PrintTable(cmpTable, true)
	return nil
}

func newAddCmd() *cobra.Command {
	cmdInst := &cobra.Command{
		Use:     "add <component1>:[version] [component2...N]",
		Short:   "Install TiDB component(s) of specific version",
		Long:    `Install some or all components of TiDB of specific version.`,
		Example: "tiup component add tidb:v3.0.8 tikv pd",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return installComponents(args)
		},
	}
	return cmdInst
}

func installComponents(specs []string) error {
	for _, spec := range specs {
		var version meta.Version
		var component string
		if strings.Contains(spec, ":") {
			parts := strings.SplitN(spec, ":", 2)
			component = parts[0]
			version = meta.Version(parts[1])
		} else {
			component = spec
		}
		manifest, err := repository.ComponentVersions(component)
		if err != nil {
			return err
		}

		err = profile.SaveVersions(component, manifest)
		if err != nil {
			return err
		}

		if version == "" {
			version = manifest.LatestStable()
		}

		// download the latest version
		err = repository.DownloadComponent(profile.ComponentsDir(), component, version)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func newRemoveCmd() *cobra.Command {
	var all bool
	cmdUnInst := &cobra.Command{
		Use:     "remove <component1>:<version> [component2...N]",
		Short:   "Uninstall TiDB component(s) of specific version",
		Long:    `Uninstall some or all components of TiDB of specific version.`,
		Example: "tiup component remove tidb:v3.0.8",
		RunE: func(cmd *cobra.Command, args []string) error {
			switch {
			case len(args) > 0:
				return removeComponents(args, all)
			case len(args) == 0 && all:
				return os.RemoveAll(profile.Path(localdata.ComponentParentDir))
			default:
				return cmd.Help()
			}
		},
	}
	cmdUnInst.Flags().BoolVar(&all, "all", false, "Remove all components or versions.")
	return cmdUnInst
}

func removeComponents(specs []string, all bool) error {
	for _, spec := range specs {
		var path string
		if strings.Contains(spec, ":") {
			parts := strings.SplitN(spec, ":", 2)
			path = profile.Path(filepath.Join(localdata.ComponentParentDir, parts[0], parts[1]))
		} else {
			if !all {
				fmt.Printf("Use `tiup remove %s --all` if you want to remove all versions.\n", spec)
				continue
			}
			path = profile.Path(filepath.Join(localdata.ComponentParentDir, spec))
		}
		err := os.RemoveAll(path)
		if err != nil {
			return err
		}
	}
	return nil
}

func newBinaryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "binary <component1>:[version]",
		Short: "Print the binary path of component of specific version",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			var component, version string
			if strings.Contains(args[0], ":") {
				parts := strings.SplitN(args[0], ":", 2)
				component, version = parts[0], parts[1]
			} else {
				component = args[0]
				installed, err := profile.InstalledVersions(component)
				if err != nil {
					return err
				}
				if len(installed) < 1 {
					return errors.Errorf("component `%s` not installed", args[0])
				}
				sort.Slice(installed, func(i, j int) bool {
					return semver.Compare(installed[i], installed[j]) < 0
				})
				version = installed[len(installed)-1]
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
