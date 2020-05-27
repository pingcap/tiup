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
	"strings"

	"github.com/pingcap-incubator/tiup/pkg/meta"
	"github.com/pingcap-incubator/tiup/pkg/repository/v1manifest"
	"github.com/pingcap-incubator/tiup/pkg/set"
	"github.com/pingcap-incubator/tiup/pkg/tui"
	"github.com/pingcap/errors"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

type listOptions struct {
	installedOnly bool
	refresh       bool
	verbose       bool
	showAll       bool
}

func newListCmd(env *meta.Environment) *cobra.Command {
	var opt listOptions
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
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			switch len(args) {
			case 0:
				result, err := showComponentList(env, opt)
				result.print()
				return err
			case 1:
				result, err := showComponentVersions(env, args[0], opt)
				result.print()
				return err
			default:
				return cmd.Help()
			}
		},
	}

	cmd.Flags().BoolVar(&opt.installedOnly, "installed", false, "List installed components only.")
	cmd.Flags().BoolVar(&opt.refresh, "refresh", false, "Refresh local components/version list cache.")
	cmd.Flags().BoolVar(&opt.verbose, "verbose", false, "Show detailed component information.")
	cmd.Flags().BoolVar(&opt.showAll, "all", false, "Show all components include hidden ones.")

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

func showComponentList(env *meta.Environment, opt listOptions) (*listResult, error) {
	local, err := v1manifest.NewManifests(env.Profile())
	if err != nil {
		return nil, err
	}

	index := new(v1manifest.Index)
	if opt.refresh {
		index, err = env.V1Repository().FetchIndexManifest()
		if err != nil {
			return nil, err
		}
	} else {
		exists, err := local.LoadManifest(index)
		if err != nil {
			return nil, errors.AddStack(err)
		}
		if !exists {
			return nil, errors.Errorf("no index manifest")
		}
	}

	installed, err := env.Profile().InstalledComponents()
	if err != nil {
		return nil, err
	}

	var cmpTable [][]string
	if opt.verbose {
		cmpTable = append(cmpTable, []string{"Name", "Owner", "Installed", "Platforms", "Description"})
	} else {
		cmpTable = append(cmpTable, []string{"Name", "Owner", "Description"})
	}

	localComponents := set.NewStringSet(installed...)
	for name, comp := range index.Components {
		if opt.installedOnly && !localComponents.Exist(name) {
			continue
		}

		if !opt.showAll && comp.Hidden {
			continue
		}

		manifest, err := env.V1Repository().FetchComponentManifest(name)
		if err != nil {
			return nil, err
		}

		if opt.verbose {
			installStatus := ""
			if localComponents.Exist(name) {
				versions, err := env.Profile().InstalledVersions(name)
				if err != nil {
					return nil, err
				}
				installStatus = strings.Join(versions, ",")
			}

			var platforms []string
			for p := range manifest.Platforms {
				platforms = append(platforms, p)
			}
			cmpTable = append(cmpTable, []string{
				name,
				comp.Owner,
				installStatus,
				strings.Join(platforms, ","),
				manifest.Description,
			})
		} else {
			cmpTable = append(cmpTable, []string{
				name,
				comp.Owner,
				manifest.Description,
			})
		}
	}

	return &listResult{
		header:   fmt.Sprintf("Available components:\n"),
		cmpTable: cmpTable,
	}, nil
}

func showComponentVersions(env *meta.Environment, component string, opt listOptions) (*listResult, error) {
	local, err := v1manifest.NewManifests(env.Profile())
	if err != nil {
		return nil, err
	}

	comp := new(v1manifest.Component)
	if opt.refresh {
		comp, err = env.V1Repository().FetchComponentManifest(component)
		if err != nil {
			return nil, err
		}
	} else {
		exists, err := local.LoadManifest(comp)
		if err != nil {
			return nil, errors.AddStack(err)
		}
		if !exists {
			return nil, errors.Errorf("no component manifest for %s", component)
		}
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
	verList := []string{}
	for v := range platforms {
		verList = append(verList, v)
	}
	sort.Slice(verList, func(p, q int) bool {
		return semver.Compare(verList[p], verList[q]) < 0
	})

	for _, v := range verList {
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
