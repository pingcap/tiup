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

	"github.com/pingcap-incubator/tiup/pkg/meta"
	"github.com/pingcap-incubator/tiup/pkg/set"
	"github.com/pingcap-incubator/tiup/pkg/tui"
	"github.com/spf13/cobra"
)

func newListCmd(env *meta.Environment) *cobra.Command {
	var (
		showInstalled bool
		refresh       bool
	)
	cmd := &cobra.Command{
		Use:   "list [component]",
		Short: "List the available TiDB components or versions",
		Long: `List the available TiDB components if you don't specify any component name,
or list the available versions of a specific component. Display a list of
local caches by default. You must use --refresh to force TiUP to fetch
the latest list from the mirror server. Use the --installed flag to hide 
components or versions which have not been installed.

  # Refresh and list all available components
  tiup list --refresh

  # List all installed components
  tiup list --installed

  # List all installed versions of TiDB
  tiup list tidb --installed`,
		RunE: func(cmd *cobra.Command, args []string) error {
			switch len(args) {
			case 0:
				result, err := showComponentList(env, showInstalled, refresh)
				result.print()
				return err
			case 1:
				result, err := showComponentVersions(env, args[0], showInstalled, refresh)
				result.print()
				return err
			default:
				return cmd.Help()
			}
		},
	}

	cmd.Flags().BoolVar(&showInstalled, "installed", false, "List installed components only.")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Refresh local components/version list cache.")
	return cmd
}

type listResult struct {
	header   string
	cmpTable [][]string
}

func (lr *listResult) print() {
	if lr == nil {
		return
	}
	fmt.Printf(lr.header)
	tui.PrintTable(lr.cmpTable, true)
}

func showComponentList(env *meta.Environment, onlyInstalled bool, refresh bool) (*listResult, error) {
	index, err := env.V1Repository().FetchIndexManifest()
	if err != nil {
		return nil, err
	}

	installed, err := env.Profile().InstalledComponents()
	if err != nil {
		return nil, err
	}

	var cmpTable [][]string
	cmpTable = append(cmpTable, []string{"Name", "Owner", "Installed", "Platforms", "Description"})

	localComponents := set.NewStringSet(installed...)
	for name, comp := range index.Components {
		if onlyInstalled && !localComponents.Exist(name) {
			continue
		}
		installStatus := ""
		if localComponents.Exist(name) {
			versions, err := env.Profile().InstalledVersions(name)
			if err != nil {
				return nil, err
			}
			installStatus = strings.Join(versions, ",")
		}

		manifest, err := env.V1Repository().FetchComponentManifest(name)
		if err != nil {
			return nil, err
		}

		platforms := []string{}
		for p := range manifest.Platforms {
			platforms = append(platforms, p)
		}

		cmpTable = append(cmpTable, []string{
			name,
			comp.Owner,
			installStatus,
			strings.Join(platforms, ", "),
			manifest.Description,
		})
	}

	return &listResult{
		header:   fmt.Sprintf("Available components:\n"),
		cmpTable: cmpTable,
	}, nil
}

func showComponentVersions(env *meta.Environment, component string, onlyInstalled bool, refresh bool) (*listResult, error) {
	comp, err := env.V1Repository().FetchComponentManifest(component)
	if err != nil {
		return nil, err
	}

	versions, err := env.Profile().InstalledVersions(component)
	if err != nil {
		return nil, err
	}
	installed := set.NewStringSet(versions...)

	var cmpTable [][]string
	cmpTable = append(cmpTable, []string{"Version", "Installed", "Release", "Platforms"})

	platforms := make(map[string][]string)
	released := make(map[string]string)

	for plat, versions := range comp.Platforms {
		for ver, verinfo := range versions {
			platforms[ver] = append(platforms[ver], plat)
			released[ver] = verinfo.Released
		}
	}

	for v := range platforms {
		installStatus := ""
		if installed.Exist(v) {
			installStatus = "YES"
		}
		cmpTable = append(cmpTable, []string{v, installStatus, released[v], strings.Join(platforms[v], ",")})
	}

	return &listResult{
		header:   fmt.Sprintf("Available versions for %s:\n", component),
		cmpTable: cmpTable,
	}, nil
}
