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
	"strings"

	"github.com/c4pt0r/tiup/pkg/set"
	"github.com/c4pt0r/tiup/pkg/tui"
	"github.com/pingcap/errors"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var (
		showInstalled bool
		refresh       bool
	)
	cmd := &cobra.Command{
		Use:   "list [component]",
		Short: "List the available TiDB components or versions",
		Long:`List the available TiDB components if you don't specify any component name.
Or list the available versions of specific component. Display a list of
local caches by default. You must use --refresh to force the tiup to fetch
the latest list from the mirror server. And you can use --installed flag
to hide components or version which don't install.

  # Refresh and list all available components
  tiup list --refresh

  # List all installed components
  tiup list --installed

  # List all installed versions of TiDB
  tiup list tidb --installed`,
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

	cmd.Flags().BoolVar(&showInstalled, "installed", false, "List installed components only.")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Refresh local components/version list cache.")
	return cmd
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
