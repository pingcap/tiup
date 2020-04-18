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
	"github.com/pingcap-incubator/tiup/pkg/set"
	"github.com/pingcap-incubator/tiup/pkg/tui"
	"github.com/pingcap/errors"
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
	fmt.Printf(lr.header)
	tui.PrintTable(lr.cmpTable, true)
}

func showComponentList(env *meta.Environment, onlyInstalled bool, refresh bool) (*listResult, error) {
	if refresh || env.Profile().Manifest() == nil {
		manifest, err := env.Repository().Manifest()
		if err != nil {
			return nil, err
		}
		err = env.Profile().SaveManifest(manifest)
		if err != nil {
			return nil, err
		}
	}

	installed, err := env.Profile().InstalledComponents()
	if err != nil {
		return nil, err
	}
	manifest := env.Profile().Manifest()
	var cmpTable [][]string
	cmpTable = append(cmpTable, []string{"Name", "Installed", "Platforms", "Description"})

	localComponents := set.NewStringSet(installed...)
	for _, comp := range manifest.Components {
		if comp.Hide {
			continue
		}
		if onlyInstalled && !localComponents.Exist(comp.Name) {
			continue
		}
		installStatus := ""
		if localComponents.Exist(comp.Name) {
			versions, err := env.Profile().InstalledVersions(comp.Name)
			if err != nil {
				return nil, err
			}
			installStatus = fmt.Sprintf("YES(%s)", strings.Join(versions, ","))
		}
		sort.Strings(comp.Platforms)
		cmpTable = append(cmpTable, []string{
			comp.Name,
			installStatus,
			strings.Join(comp.Platforms, ","),
			comp.Desc,
		})
	}

	return &listResult{
		header:   fmt.Sprintf("Available components (Last Modified: %s):\n", manifest.Modified),
		cmpTable: cmpTable,
	}, nil
}

func showComponentVersions(env *meta.Environment, component string, onlyInstalled bool, refresh bool) (*listResult, error) {
	if refresh || env.Profile().Versions(component) == nil {
		manifest, err := env.Repository().ComponentVersions(component)
		if err != nil {
			return nil, errors.Trace(err)
		}
		err = env.Profile().SaveVersions(component, manifest)
		if err != nil {
			return nil, err
		}
	}

	versions, err := env.Profile().InstalledVersions(component)
	if err != nil {
		return nil, err
	}
	manifest := env.Profile().Versions(component)

	var cmpTable [][]string
	cmpTable = append(cmpTable, []string{"Version", "Installed", "Release:", "Platforms"})

	installed := set.NewStringSet(versions...)
	released := manifest.Versions
	if manifest.Nightly != nil {
		released = append(released, *manifest.Nightly)
	}
	for _, ver := range released {
		version := ver.Version.String()
		if onlyInstalled && !installed.Exist(version) {
			continue
		}
		installStatus := ""
		if installed.Exist(version) {
			installStatus = "YES"
		}
		sort.Strings(ver.Platforms)
		cmpTable = append(cmpTable, []string{
			version,
			installStatus,
			ver.Date,
			strings.Join(ver.Platforms, ","),
		})
	}

	return &listResult{
		header:   fmt.Sprintf("Available versions for %s (Last Modified: %s):\n", component, manifest.Modified),
		cmpTable: cmpTable,
	}, nil
}
