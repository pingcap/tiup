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
	"sort"
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/repository"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/tui"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

type listOptions struct {
	installedOnly bool
	verbose       bool
	showAll       bool
	mirrorList    []string
}

func newListCmd() *cobra.Command {
	var opt listOptions
	cmd := &cobra.Command{
		Use:   "list [component]",
		Short: "List the available TiDB components or versions",
		Long: `List the available TiDB components if you don't specify any component name,
or list the available versions of a specific component. Display a list of
local caches by default. Use the --installed flag to hide
components or versions which have not been installed.

  # List all installed components
  tiup list --installed

  # List all installed versions of TiDB
  tiup list tidb --installed`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			teleCommand = cmd.CommandPath()

			if len(opt.mirrorList) == 0 {
				for _, m := range tiupC.ListMirrors() {
					opt.mirrorList = append(opt.mirrorList, m.Name)
				}

			}

			switch len(args) {
			case 0:
				result, err := showComponentList(opt)
				result.print()
				return err
			case 1:
				result, err := showComponentVersions(args[0], opt)
				result.print()
				return err
			default:
				return cmd.Help()
			}
		},
	}

	cmd.Flags().BoolVar(&opt.installedOnly, "installed", false, "List installed components only.")
	cmd.Flags().StringSliceVar(&opt.mirrorList, "mirrors", []string{}, "Specify the components that display mirrors.")
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

func showComponentList(opt listOptions) (*listResult, error) {

	var (
		cmpTable   [][]string
		components [][]string
		err        error
	)

	if opt.verbose {
		cmpTable = append(cmpTable, []string{"Mirror", "Name", "Owner", "Installed", "Platforms", "Description"})
	} else {
		cmpTable = append(cmpTable, []string{"Mirror", "Name", "Owner", "Description"})
	}

	// single mirror
	if len(tiupC.ListMirrors()) == 0 {
		env := environment.GlobalEnv()
		components, err = showComponentListOfMirror(environment.Mirror(), env.V1Repository(), opt)
	} else {
		// multi-mirror
		for _, name := range opt.mirrorList {

			c, err := showComponentListOfMirror(name, tiupC.GetRepository(name), opt)
			if err != nil {
				log.Warnf("get component list from mirror %s failed", name)
				continue
			}
			components = append(components, c...)
		}
	}

	if err != nil {
		return nil, err
	}

	return &listResult{
		header:   "Available components:\n",
		cmpTable: append(cmpTable, components...),
	}, nil
}

func showComponentListOfMirror(mirror string, repo *repository.V1Repository, opt listOptions) ([][]string, error) {
	if !opt.installedOnly {
		err := repo.UpdateComponentManifests()
		if err != nil {
			tui.ColorWarningMsg.Fprint(os.Stderr, "Warn: Update component manifest failed, err_msg=[", err.Error(), "]\n")
		}
	}

	installed, err := repo.Local().InstalledComponents()
	if err != nil {
		return nil, err
	}

	var cmpTable [][]string

	index := v1manifest.Index{}
	_, exists, err := repo.Local().LoadManifest(&index)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.Errorf("unreachable: index.json not found in manifests directory")
	}

	localComponents := set.NewStringSet(installed...)
	compIDs := []string{}
	components := index.ComponentList()
	for id := range components {
		compIDs = append(compIDs, id)
	}
	sort.Strings(compIDs)
	for _, id := range compIDs {
		comp := components[id]
		if opt.installedOnly && !localComponents.Exist(id) {
			continue
		}

		if (!opt.installedOnly && !opt.showAll) && comp.Hidden {
			continue
		}

		filename := v1manifest.ComponentManifestFilename(id)
		manifest, err := repo.Local().LoadComponentManifest(&comp, filename)
		if err != nil {
			return nil, err
		}

		if manifest == nil {
			continue
		}

		if opt.verbose {
			installStatus := ""
			if localComponents.Exist(id) {
				versions, err := repo.Local().InstalledVersions(id)
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
				mirror,
				id,
				comp.Owner,
				installStatus,
				strings.Join(platforms, ","),
				manifest.Description,
			})
		} else {
			cmpTable = append(cmpTable, []string{
				mirror,
				id,
				comp.Owner,
				manifest.Description,
			})
		}
	}

	return cmpTable, nil
}

func showComponentVersions(component string, opt listOptions) (*listResult, error) {

	var (
		components [][]string
		err        error
	)

	cmpTable := [][]string{
		{"Mirror", "Version", "Installed", "Release", "Platforms"},
	}

	// single mirror
	if len(tiupC.ListMirrors()) == 0 {
		env := environment.GlobalEnv()
		components, err = showComponentVersionsOfMirror(environment.Mirror(), env.V1Repository(), component, opt)
	} else {
		// multi-mirror
		// multi-mirror
		for _, name := range opt.mirrorList {

			c, err := showComponentVersionsOfMirror(name, tiupC.GetRepository(name), component, opt)
			if err != nil {
				log.Debugf("Not found %s in mirror %s", component, name)
				continue
			}
			components = append(components, c...)
		}
	}

	if err != nil {
		return nil, err
	}

	return &listResult{
		header:   "Available components:\n",
		cmpTable: append(cmpTable, components...),
	}, nil
}

func showComponentVersionsOfMirror(mirror string, repo *repository.V1Repository, component string, opt listOptions) ([][]string, error) {
	var comp *v1manifest.Component
	var err error
	if opt.installedOnly {
		comp, err = repo.LocalComponentManifest(component, false)
	} else {
		comp, err = repo.GetComponentManifest(component, false)
	}
	if err != nil {
		return nil, errors.Annotate(err, "failed to fetch component")
	}

	versions, err := repo.Local().InstalledVersions(component)
	if err != nil {
		return nil, err
	}
	installed := set.NewStringSet(versions...)

	var cmpTable [][]string

	platforms := make(map[string][]string)
	released := make(map[string]string)

	for plat := range comp.Platforms {
		versions := comp.VersionList(plat)
		for ver, verinfo := range versions {
			if ver == comp.Nightly {
				key := fmt.Sprintf("%s -> %s", utils.NightlyVersionAlias, comp.Nightly)
				platforms[key] = append(platforms[key], plat)
				released[key] = verinfo.Released
			}
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
		} else if opt.installedOnly {
			continue
		}
		cmpTable = append(cmpTable, []string{mirror, v, installStatus, released[v], strings.Join(platforms[v], ",")})
	}

	return cmpTable, nil
}
