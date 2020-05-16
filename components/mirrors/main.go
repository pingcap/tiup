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

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pingcap-incubator/tiup/pkg/localdata"
	"github.com/pingcap-incubator/tiup/pkg/meta"
	"github.com/pingcap-incubator/tiup/pkg/repository"
	"github.com/pingcap-incubator/tiup/pkg/repository/v0manifest"
	"github.com/pingcap-incubator/tiup/pkg/set"
	"github.com/pingcap-incubator/tiup/pkg/utils"
	"github.com/pingcap-incubator/tiup/pkg/version"
	"github.com/pingcap/errors"
	"github.com/spf13/cobra"
)

type mirrorsOptions struct {
	archs     []string
	oss       []string
	versions  []string
	full      bool
	comps     map[string]*[]string
	overwrite bool
}

func main() {
	if err := execute(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}

func execute() error {
	cobra.EnableCommandSorting = false
	if wd := os.Getenv(localdata.EnvNameWorkDir); wd != "" {
		_ = os.Chdir(wd)
	}
	options := mirrorsOptions{
		comps: map[string]*[]string{},
	}

	mirror := repository.NewMirror(meta.Mirror(), repository.MirrorOptions{})
	repo, err := repository.NewRepository(mirror, repository.Options{
		SkipVersionCheck:  true,
		DisableDecompress: true,
	})
	if err != nil {
		return err
	}
	defer repo.Close()

	manifest, err := repo.Manifest()
	if err != nil {
		return err
	}

	rootCmd := &cobra.Command{
		Use: "tiup mirrors <target-dir> [global version]",
		Example: `  tiup mirrors local-path --arch amd64,arm --os linux,darwin    # Specify the architectures and OSs
  tiup mirrors local-path --full                                # Build a full local mirrors
  tiup mirrors local-path --tikv v4                             # Specify the version via prefix
  tiup mirrors local-path --tidb all --pd all                   # Download all version for specific component`,
		Short:        "Build a local mirrors and download all selected components",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			for _, v := range args[1:] {
				for _, comp := range manifest.Components {
					if err := setVersion(repo, comp.Name, options.comps[comp.Name], v); err != nil {
						return err
					}
				}
			}
			if err := download(args[0], repo, manifest, options); err != nil {
				return err
			}
			return copyInstallScript(args[0])
		},
	}

	rootCmd.Flags().SortFlags = false
	rootCmd.Flags().BoolVar(&options.overwrite, "overwrite", false, "Overwrite the exists tarball")
	rootCmd.Flags().BoolVarP(&options.full, "full", "f", false, "Build a full mirrors repository")
	rootCmd.Flags().StringSliceVarP(&options.archs, "arch", "a", []string{"amd64", "arm64"}, "Specify the downloading architecture")
	rootCmd.Flags().StringSliceVarP(&options.oss, "os", "o", []string{"linux", "darwin"}, "Specify the downloading os")

	for _, comp := range manifest.Components {
		options.comps[comp.Name] = new([]string)
		rootCmd.Flags().StringSliceVar(options.comps[comp.Name], comp.Name, nil, "Specify the versions for component "+comp.Name)
	}

	return rootCmd.Execute()
}

func setVersion(repo *repository.Repository, component string, versions *[]string, version string) error {
	v, err := suggestVersion(repo, component, version)
	if err != nil {
		return err
	}
	if *versions == nil {
		*versions = []string{v}
		return nil
	}

	for _, ver := range *versions {
		if v == ver {
			return nil
		}
	}
	*versions = append(*versions, v)
	return nil
}

func suggestVersion(repo *repository.Repository, component, version string) (string, error) {
	switch component {
	case "alertmanager":
		return "v0.17.0", nil
	case "blackbox_exporter":
		return "v0.12.0", nil
	case "node_exporter":
		return "v0.17.0", nil
	case "pushgateway":
		return "v0.7.0", nil
	}
	vm, err := repo.ComponentVersions(component)
	if err != nil {
		return "", err
	}
	if v, found := vm.FindVersion(v0manifest.Version(version)); found {
		return v.Version.String(), nil
	}
	v, err := repo.LatestStableVersion(component)
	if err != nil {
		return "", err
	}
	return v.String(), nil
}

func downloadResource(mirror repository.Mirror, targetDir, name string, overwrite bool) error {
	tarFile := name + ".tar.gz"
	shaFile := name + ".sha1"
	if !overwrite && utils.IsExist(filepath.Join(targetDir, shaFile)) {
		fmt.Printf("Skip to download resource: %s\n", name)
		return nil
	}

	if err := mirror.Download(tarFile, targetDir); err != nil {
		return err
	}
	return mirror.Download(shaFile, targetDir)
}

func download(targetDir string, repo *repository.Repository, m *v0manifest.ComponentManifest, options mirrorsOptions) error {
	if utils.IsNotExist(targetDir) {
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return err
		}
	}

	fmt.Println("Arch", options.archs)
	fmt.Println("OS", options.oss)

	if len(options.oss) == 0 || len(options.archs) == 0 {
		return nil
	}

	filename := func(name string) string {
		return filepath.Join(targetDir, name)
	}

	writeJSON := func(file string, data interface{}) error {
		jsonData, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return err
		}
		return ioutil.WriteFile(file, jsonData, os.ModePerm)
	}

	if err := writeJSON(filename(repository.ManifestFileName), m); err != nil {
		return err
	}

	for name, versions := range options.comps {
		componentInfo, err := repo.ComponentVersions(name)
		if err != nil {
			return err
		}

		vs := set.NewStringSet(*versions...)
		var newCompInfo *v0manifest.VersionManifest
		if options.full {
			newCompInfo = componentInfo
		} else {
			if len(vs) < 1 {
				continue
			}
			newCompInfo = &v0manifest.VersionManifest{
				Description: componentInfo.Description,
				Modified:    componentInfo.Modified,
			}
			if vs.Exist(version.NightlyVersion) {
				newCompInfo.Nightly = componentInfo.Nightly
			}
		}

		checkVersion := func(version v0manifest.Version) bool {
			if options.full || vs.Exist("all") || vs.Exist(version.String()) {
				return true
			}
			// prefix match
			for v := range vs {
				if strings.HasPrefix(version.String(), v) {
					return true
				}
			}
			return false
		}
		err = componentInfo.IterVersion(func(versionInfo v0manifest.VersionInfo) error {
			if !checkVersion(versionInfo.Version) {
				return nil
			}
			if !options.full {
				newCompInfo.Versions = append(newCompInfo.Versions, versionInfo)
			}
			for _, goos := range options.oss {
				for _, goarch := range options.archs {
					if !versionInfo.IsSupport(goos, goarch) {
						fmt.Printf("The `%s:%s` donesn't %s/%s, skipped\n", name, versionInfo.Version, goos, goarch)
						continue
					}
					name := fmt.Sprintf("%s-%s-%s-%s", name, versionInfo.Version, goos, goarch)
					if err := downloadResource(repo.Mirror(), targetDir, name, options.overwrite); err != nil {
						return errors.Annotatef(err, "download resource: %s", name)
					}
				}
			}
			return nil
		})
		if err != nil {
			return err
		}

		if err := writeJSON(filename(fmt.Sprintf("tiup-component-%s.index", name)), newCompInfo); err != nil {
			return err
		}
	}

	// download tiup itself
	for _, os := range options.oss {
		for _, goarch := range options.archs {
			name := fmt.Sprintf("tiup-%s-%s", os, goarch)
			if err := downloadResource(repo.Mirror(), targetDir, name, options.overwrite); err != nil {
				return errors.Annotatef(err, "download resource: %s", name)
			}
		}
	}

	return nil
}

func copyInstallScript(targetDir string) error {
	home := os.Getenv(localdata.EnvNameComponentInstallDir)
	if err := utils.Copy(path.Join(home, "install.sh"), path.Join(targetDir, "install.sh")); err != nil {
		return err
	}
	return utils.Copy(path.Join(home, "local_install.sh"), path.Join(targetDir, "local_install.sh"))
}
