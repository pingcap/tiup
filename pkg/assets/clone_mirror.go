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

package assets

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pingcap-incubator/tiup/pkg/version"
	"golang.org/x/mod/semver"

	"github.com/pingcap/errors"

	"github.com/pingcap-incubator/tiup/pkg/repository"
	"github.com/pingcap-incubator/tiup/pkg/repository/v1manifest"
	"github.com/pingcap-incubator/tiup/pkg/set"
	"github.com/pingcap-incubator/tiup/pkg/utils"
)

// CloneOptions represents the options of clone a remote mirror
type CloneOptions struct {
	Archs      []string
	OSs        []string
	Versions   []string
	Full       bool
	Components map[string]*[]string
}

// CloneMirror clones a local mirror from the remote repository
func CloneMirror(repo *repository.V1Repository, components []string, targetDir string, selectedVersions []string, options CloneOptions) error {
	if utils.IsNotExist(targetDir) {
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return err
		}
	}

	// Temporary directory is used to save the unverified tarballs
	tmpDir := filepath.Join(targetDir, fmt.Sprintf("_tmp_%d", time.Now().UnixNano()))
	if utils.IsNotExist(tmpDir) {
		if err := os.MkdirAll(tmpDir, 0755); err != nil {
			return err
		}
	}
	defer os.RemoveAll(tmpDir)

	fmt.Println("Arch", options.Archs)
	fmt.Println("OS", options.OSs)

	if len(options.OSs) == 0 || len(options.Archs) == 0 {
		return nil
	}

	manifests := map[string]*v1manifest.Component{}
	for _, name := range components {
		manifest, err := repo.FetchComponentManifest(name)
		if err != nil {
			return errors.Annotatef(err, "fetch component '%s' manifest", name)
		}

		vs := combineVersions(options.Components[name], manifest, options.OSs, options.Archs, selectedVersions)
		var newManifest *v1manifest.Component
		if options.Full {
			newManifest = manifest
		} else {
			if len(vs) < 1 {
				continue
			}
			// TODO: support nightly version
			newManifest = &v1manifest.Component{
				SignedBase:  manifest.SignedBase,
				ID:          manifest.ID,
				Name:        manifest.Name,
				Description: manifest.Description,
				Platforms:   map[string]map[string]v1manifest.VersionItem{},
			}
		}

		for _, goos := range options.OSs {
			for _, goarch := range options.Archs {
				platform := repository.PlatformString(goos, goarch)
				versions, found := manifest.Platforms[platform]
				if !found {
					fmt.Printf("The component '%s' donesn't %s/%s, skipped\n", name, goos, goarch)
				}
				for version, versionItem := range versions {
					if !checkVersion(options, vs, version) {
						continue
					}
					if !options.Full {
						newVersions, found := newManifest.Platforms[platform]
						if !found {
							newVersions = map[string]v1manifest.VersionItem{}
							newManifest.Platforms[platform] = newVersions
						}
						newVersions[version] = versionItem
					}
					if err := download(targetDir, tmpDir, repo, &versionItem); err != nil {
						return errors.Annotatef(err, "download resource: %s", name)
					}
				}
			}
		}
		manifests[name] = newManifest
	}

	// TODO: write all manifests

	return writeLocalInstallScript(filepath.Join(targetDir, "local_install.sh"))
}

func download(targetDir, tmpDir string, repo *repository.V1Repository, item *v1manifest.VersionItem) error {
	err := repo.Mirror().Download(item.URL, tmpDir)
	if err != nil {
		return err
	}
	hashes, n, err := repository.HashFile(tmpDir, item.URL)
	if err != nil {
		return errors.AddStack(err)
	}
	if uint(n) != item.Length {
		return errors.Errorf("file length mismatch, expected: %d, got: %v", item.Length, n)
	}
	for algo, hash := range item.Hashes {
		h, found := hashes[algo]
		if !found {
			continue
		}
		if h != hash {
			return errors.Errorf("file %s hash mismatch, expected: %s, got: %s", algo, hash, h)
		}
	}

	// Move file to target directory if hashes pass verify.
	return os.Rename(filepath.Join(tmpDir, item.URL), filepath.Join(targetDir, item.URL))
}

func checkVersion(options CloneOptions, versions set.StringSet, version string) bool {
	if options.Full || versions.Exist("all") || versions.Exist(version) {
		return true
	}
	// prefix match
	for v := range versions {
		if strings.HasPrefix(version, v) {
			return true
		}
	}
	return false
}

func combineVersions(versions *[]string, manifest *v1manifest.Component, oss, archs, selectedVersions []string) set.StringSet {
	if (versions == nil || len(*versions) < 1) && len(selectedVersions) < 1 {
		return nil
	}

	switch manifest.Name {
	case "alertmanager":
		return set.NewStringSet("v0.17.0")
	case "blackbox_exporter":
		return set.NewStringSet("v0.12.0")
	case "node_exporter":
		return set.NewStringSet("v0.17.0")
	case "pushgateway":
		return set.NewStringSet("v0.7.0")
	}

	result := set.NewStringSet()
	if versions != nil && len(*versions) > 0 {
		result = set.NewStringSet(*versions...)
	}

	for _, os := range oss {
		for _, arch := range archs {
			platform := repository.PlatformString(os, arch)
			versions, found := manifest.Platforms[platform]
			if !found {
				continue
			}
			for _, selectedVersion := range selectedVersions {
				_, found := versions[selectedVersion]
				if !found {
					// Use the latest stable versionS if the selected version doesn't exist in specific platform
					var latest string
					for v := range versions {
						if strings.Contains(v, version.NightlyVersion) {
							continue
						}
						if latest == "" || semver.Compare(v, latest) > 0 {
							latest = v
						}
					}
					if latest == "" {
						continue
					}
					selectedVersion = latest
				}
				if !result.Exist(selectedVersion) {
					result.Insert(selectedVersion)
				}
			}
		}
	}
	return result
}
