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
	"golang.org/x/mod/semver"
)

var (
	defaultMirror = "http://118.24.4.54/tiup/"
	manifestPath  = "manifest/tiup-manifest.index"
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
	cmdComponent.AddCommand(newAddCmd())
	cmdComponent.AddCommand(newRemoveCmd())
	cmdComponent.AddCommand(newBinaryCmd())
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

func loadInstalledVersions(component string) ([]string, error) {
	path, err := profile.Path(fmt.Sprintf("components/" + component))
	if err != nil {
		return nil, err
	}
	if utils.IsNotExist(path) {
		return nil, nil
	}

	fileInfos, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, errors.Trace(err)
	}
	var versions []string
	for _, fi := range fileInfos {
		versions = append(versions, fi.Name())
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

	installed := set.NewStringSet(versions...)
	for _, ver := range manifest.Versions {
		if onlyInstalled && !installed.Exist(ver.Version) {
			continue
		}
		installStatus := ""
		if installed.Exist(ver.Version) {
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
	// TODO: use mirror from configuration or command-line args
	mirror := meta.NewMirror(defaultMirror)
	if err := mirror.Open(); err != nil {
		return errors.Trace(err)
	}
	defer mirror.Close()

	repo := meta.NewRepository(mirror)
	for _, spec := range specs {
		if strings.Contains(spec, ":") {
			parts := strings.SplitN(spec, ":", 2)
			if err := repo.Download(parts[0], parts[1]); err != nil {
				return err
			}
		} else {
			manifest, err := repo.ComponentVersions(spec)
			if err != nil {
				return errors.Trace(err)
			}
			if len(manifest.Versions) < 1 {
				return errors.Errorf("component %s does not exist", spec)
			}

			// cache the version manifest and ignore the error
			_ = profile.WriteJSON(versionManifestFile(spec), manifest)

			// download the latest version
			err = repo.Download(spec, manifest.Versions[len(manifest.Versions)-1].Version)
			if err != nil {
				return errors.Trace(err)
			}
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
				compsDir, err := profile.Path("components")
				if err != nil {
					return err
				}
				return os.RemoveAll(compsDir)
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
		var err error
		if strings.Contains(spec, ":") {
			parts := strings.SplitN(spec, ":", 2)
			path, err = profile.Path(filepath.Join("components", parts[0], parts[1]))
		} else {
			if !all {
				fmt.Printf("Use `tiup remove %s --all` if you want to remove all versions.\n", spec)
				continue
			}
			path, err = profile.Path(filepath.Join("components", spec))
		}
		if err != nil {
			return err
		}
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return nil
}

func newBinaryCmd() *cobra.Command {
	var all bool
	cmdUnInst := &cobra.Command{
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
				installed, err := loadInstalledVersions(args[0])
				if err != nil {
					return err
				}
				if len(installed) < 1 {
					return errors.Errorf("Component `%s` not installed", args[0])
				}
				sort.Slice(installed, func(i, j int) bool {
					return semver.Compare(installed[i], installed[j]) < 0
				})
				component, version = args[0], installed[len(installed)-1]
			}

			manifest, err := loadComponentVersions(component)
			if err != nil {
				return err
			}
			var entry string
			for _, v := range manifest.Versions {
				if v.Version == version {
					entry = v.Entry
				}
			}
			if entry == "" {
				return errors.Errorf("Cannot found entry for %s:%s", component, version)
			}
			path, err := profile.Path(filepath.Join("components", component, version))
			if err != nil {
				return err
			}
			fmt.Println(filepath.Join(path, entry))
			return nil
		},
	}

	cmdUnInst.Flags().BoolVar(&all, "all", false, "Remove all components or versions.")
	return cmdUnInst
}
