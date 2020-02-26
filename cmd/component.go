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
	"path/filepath"
	"sort"
	"strings"

	"github.com/c4pt0r/tiup/pkg/meta"
	"github.com/c4pt0r/tiup/pkg/profile"
	"github.com/c4pt0r/tiup/pkg/set"
	"github.com/c4pt0r/tiup/pkg/tui"
	"github.com/c4pt0r/tiup/pkg/utils"
	"github.com/pingcap/errors"
	"github.com/spf13/cobra"
)

var (
	componentListURL      = "https://repo.hoshi.at/tmp/components.json"
	defaultMirror         = "http://118.24.4.54/tiup/"
	installedListFilename = "installed.json"
	specifiedHomeEnvKey   = "TIUP_HOME"

	manifestPath = "manifest/tiup-manifest.index"
)

func newComponentCmd() *cobra.Command {
	cmdComponent := &cobra.Command{
		Use:   "component",
		Short: "Manage TiDB related components",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmdComponent.AddCommand(newListComponentCmd())
	cmdComponent.AddCommand(newInstCmd())
	cmdComponent.AddCommand(newUnInstCmd())
	return cmdComponent
}

func versionManifestFile(component string) string {
	return fmt.Sprintf("manifest/tiup-component-%s.index", component)
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
				// tiup component list
				path, err := profile.Path(manifestPath)
				if err != nil {
					return err
				}
				if refresh || utils.IsNotExist(path) {
					err := refreshComponentList()
					if err != nil {
						return err
					}
				}
				return showComponentList(showInstalled)
			case 1:
				// tiup component list [component]
				component := args[0]
				path, err := profile.Path(versionManifestFile(component))
				if err != nil {
					return err
				}
				if refresh || utils.IsNotExist(path) {
					err := refreshComponentVersions(component)
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

func refreshComponentList() error {
	// TODO: use mirror from configuration or command-line args
	mirror := meta.NewMirror(defaultMirror)
	if err := mirror.Open(); err != nil {
		return errors.Trace(err)
	}
	defer mirror.Close()

	repo := meta.NewRepository(mirror)
	manifest, err := repo.Components()
	if err != nil {
		return errors.Trace(err)
	}

	return profile.WriteJSON(manifestPath, manifest)
}

func refreshComponentVersions(component string) error {
	// TODO: use mirror from configuration or command-line args
	mirror := meta.NewMirror(defaultMirror)
	if err := mirror.Open(); err != nil {
		return errors.Trace(err)
	}
	defer mirror.Close()

	repo := meta.NewRepository(mirror)
	manifest, err := repo.ComponentVersions(component)
	if err != nil {
		return errors.Trace(err)
	}

	return profile.WriteJSON(versionManifestFile(component), manifest)
}

func loadCachedManifest() (*meta.ComponentManifest, error) {
	var manifest meta.ComponentManifest
	if err := profile.ReadJSON(manifestPath, &manifest); err != nil {
		return nil, errors.Trace(err)
	}

	return &manifest, nil
}

func showComponentList(onlyInstalled bool) error {
	installed, err := getInstalledList()
	if err != nil {
		return err
	}

	manifest, err := loadCachedManifest()
	if err != nil {
		return err
	}

	var cmpTable [][]string
	cmpTable = append(cmpTable, []string{"Name", "Installed", "Platforms", "Desc"})

	localComponents := set.NewStringSet(installed...)
	for _, comp := range manifest.Components {
		if onlyInstalled && !localComponents.Exist(comp.Name) {
			continue
		}
		installStatus := ""
		if localComponents.Exist(comp.Name) {
			installStatus = "yes"
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

func loadInstalledVersions(component string) (set.StringSet, error) {
	path, err := profile.Path(fmt.Sprintf("components/" + component))
	if err != nil {
		return nil, err
	}
	if utils.IsNotExist(path) {
		return set.StringSet{}, nil
	}

	fileInfos, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, errors.Trace(err)
	}

	versions := set.StringSet{}
	for _, fi := range fileInfos {
		versions.Insert(fi.Name())
	}
	return versions, nil
}

func loadComponentVersions(component string) (*meta.VersionManifest, error) {
	var manifest meta.VersionManifest
	err := profile.ReadJSON(versionManifestFile(component), &manifest)
	if err != nil {
		return nil, err
	}
	return &manifest, nil
}

func showComponentVersions(component string, onlyInstalled bool) error {
	versions, err := loadInstalledVersions(component)
	if err != nil {
		return err
	}

	manifest, err := loadComponentVersions(component)
	if err != nil {
		return err
	}

	var cmpTable [][]string
	cmpTable = append(cmpTable, []string{"Version", "Installed", "Date", "Platforms"})

	for _, ver := range manifest.Versions {
		if onlyInstalled && !versions.Exist(ver.Version) {
			continue
		}
		installStatus := ""
		if versions.Exist(ver.Version) {
			installStatus = "yes"
		}
		cmpTable = append(cmpTable, []string{
			ver.Version,
			installStatus,
			ver.Date,
			strings.Join(ver.Platforms, ","),
		})
	}

	fmt.Printf("Available versions for %s (Last Modified: %s):\n", component, manifest.Modified)
	tui.PrintTable(cmpTable, true)
	return nil
}

func newInstCmd() *cobra.Command {
	var (
	//version       string
	//componentList []string
	)

	cmdInst := &cobra.Command{
		Use:     "install <component1> [component2...N] <version>",
		Short:   "Install TiDB component(s) of specific version",
		Long:    `Install some or all components of TiDB of specific version.`,
		Example: "tiup component install tidb-core v3.0.8",
		Args: func(cmd *cobra.Command, args []string) error {
			argsLen := len(args)
			//var err error
			switch argsLen {
			case 0:
				return cmd.Help()
			case 1: // version unspecified, use stable latest as default
				//currChan, err := meta.ReadVersionFile()
				//if os.IsNotExist(err) {
				//	fmt.Println("default version not set, using latest stable.")
				//	compMeta, err := meta.ReadComponentList()
				//	if os.IsNotExist(err) {
				//		fmt.Println("no available component list, try `tiup component list --refresh` to get latest online list.")
				//		return nil
				//	} else if err != nil {
				//		return err
				//	}
				//	version = compMeta.Stable
				//} else if err != nil {
				//	return err
				//}
				//version = currChan.Ver
				//componentList = args
			default:
				//version, err = utils.FmtVer(args[argsLen-1])
				//if err != nil {
				//	return err
				//}
				//componentList = args[:argsLen-1]
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("not implement")
		},
	}
	return cmdInst
}

func getInstalledList() ([]string, error) {
	profDir, err := profile.Dir()
	if err != nil {
		return nil, errors.Trace(err)
	}
	compDir := filepath.Join(profDir, "components")
	fileInfos, err := ioutil.ReadDir(compDir)
	if err != nil && os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.Trace(err)
	}
	var components []string
	for _, fi := range fileInfos {
		if !fi.IsDir() {
			continue
		}
		components = append(components, fi.Name())
	}
	sort.Strings(components)
	return components, nil
}

func newUnInstCmd() *cobra.Command {
	var (
		version       string
		componentList []string
	)

	cmdUnInst := &cobra.Command{
		Use:     "uninstall <component1> [component2...N] <version>",
		Short:   "Uninstall TiDB component(s) of specific version",
		Long:    `Uninstall some or all components of TiDB of specific version.`,
		Example: "tiup component uninstall tidb-core v3.0.8",
		Args: func(cmd *cobra.Command, args []string) error {
			argsLen := len(args)
			if argsLen < 2 {
				cmd.Help()
				return nil
			}
			var err error
			version, err = utils.FmtVer(args[argsLen-1])
			if err != nil {
				return err
			}
			componentList = args[:argsLen-1]
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = version
			_ = componentList
			return errors.New("not implement")
		},
	}

	return cmdUnInst
}
