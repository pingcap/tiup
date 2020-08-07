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

package repository

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pingcap/tiup/pkg/cluster/template/install"

	"github.com/pingcap/errors"
	ru "github.com/pingcap/tiup/pkg/repository/utils"
	"github.com/pingcap/tiup/pkg/repository/v0manifest"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/pingcap/tiup/pkg/version"
	"golang.org/x/mod/semver"
)

// CloneOptions represents the options of clone a remote mirror
type CloneOptions struct {
	Archs      []string
	OSs        []string
	Versions   []string
	Full       bool
	Components map[string]*[]string
	Prefix     bool
}

// CloneMirror clones a local mirror from the remote repository
func CloneMirror(repo *V1Repository, components []string, targetDir string, selectedVersions []string, options CloneOptions) error {
	fmt.Printf("Start to clone mirror, targetDir is %s, selectedVersions are [%s]\n", targetDir, strings.Join(selectedVersions, ","))
	fmt.Println("If this does not meet expectations, please abort this process, read `tiup mirror clone --help` and run again")

	if utils.IsNotExist(targetDir) {
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return err
		}
	}

	// Temporary directory is used to save the unverified tarballs
	tmpDir := filepath.Join(targetDir, fmt.Sprintf("_tmp_%d", time.Now().UnixNano()))
	keyDir := filepath.Join(targetDir, "keys")

	if utils.IsNotExist(tmpDir) {
		if err := os.MkdirAll(tmpDir, 0755); err != nil {
			return err
		}
	}
	if utils.IsNotExist(keyDir) {
		if err := os.MkdirAll(keyDir, 0755); err != nil {
			return err
		}
	}
	defer os.RemoveAll(tmpDir)

	fmt.Println("Arch", options.Archs)
	fmt.Println("OS", options.OSs)

	if len(options.OSs) == 0 || len(options.Archs) == 0 {
		return nil
	}

	var (
		initTime = time.Now()
		expirsAt = initTime.Add(50 * 365 * 24 * time.Hour)
		root     = v1manifest.NewRoot(initTime)
		index    = v1manifest.NewIndex(initTime)
	)

	// All offline expires at 50 years to prevent manifests stale
	root.SetExpiresAt(expirsAt)
	index.SetExpiresAt(expirsAt)

	keys := map[string][]*v1manifest.KeyInfo{}
	for _, ty := range []string{
		v1manifest.ManifestTypeRoot,
		v1manifest.ManifestTypeIndex,
		v1manifest.ManifestTypeSnapshot,
		v1manifest.ManifestTypeTimestamp,
	} {
		if err := v1manifest.GenAndSaveKeys(keys, ty, int(v1manifest.ManifestsConfig[ty].Threshold), keyDir); err != nil {
			return err
		}
	}

	// initial manifests
	manifests := map[string]v1manifest.ValidManifest{
		v1manifest.ManifestTypeRoot:  root,
		v1manifest.ManifestTypeIndex: index,
	}
	signedManifests := make(map[string]*v1manifest.Manifest)

	genkey := func() (string, *v1manifest.KeyInfo, error) {
		priv, err := v1manifest.GenKeyInfo()
		if err != nil {
			return "", nil, err
		}

		id, err := priv.ID()
		if err != nil {
			return "", nil, err
		}

		return id, priv, nil
	}

	// Initialize the index manifest
	ownerkeyID, ownerkeyInfo, err := genkey()
	if err != nil {
		return errors.Trace(err)
	}
	// save owner key
	if err := v1manifest.SaveKeyInfo(ownerkeyInfo, "pingcap", keyDir); err != nil {
		return errors.Trace(err)
	}

	ownerkeyPub, err := ownerkeyInfo.Public()
	if err != nil {
		return errors.Trace(err)
	}
	index.Owners["pingcap"] = v1manifest.Owner{
		Name: "PingCAP",
		Keys: map[string]*v1manifest.KeyInfo{
			ownerkeyID: ownerkeyPub,
		},
		Threshold: 1,
	}

	snapshot := v1manifest.NewSnapshot(initTime)
	snapshot.SetExpiresAt(expirsAt)

	componentManifests, err := cloneComponents(repo, components, selectedVersions, targetDir, tmpDir, options)
	if err != nil {
		return err
	}

	for name, component := range componentManifests {
		component.SetExpiresAt(expirsAt)
		fname := fmt.Sprintf("%s.json", name)
		// TODO: support external owner
		signedManifests[component.ID], err = v1manifest.SignManifest(component, ownerkeyInfo)
		if err != nil {
			return err
		}
		index.Components[component.ID] = v1manifest.ComponentItem{
			Owner: "pingcap",
			URL:   fmt.Sprintf("/%s", fname),
		}
	}

	// sign index and snapshot
	signedManifests[v1manifest.ManifestTypeIndex], err = v1manifest.SignManifest(index, keys[v1manifest.ManifestTypeIndex]...)
	if err != nil {
		return err
	}

	// snapshot and timestamp are the last two manifests to be initialized

	// Initialize timestamp
	timestamp := v1manifest.NewTimestamp(initTime)
	timestamp.SetExpiresAt(expirsAt)

	manifests[v1manifest.ManifestTypeTimestamp] = timestamp
	manifests[v1manifest.ManifestTypeSnapshot] = snapshot

	// Initialize the root manifest
	for _, m := range manifests {
		if err := root.SetRole(m, keys[m.Base().Ty]...); err != nil {
			return err
		}
	}

	// Sign root
	signedManifests[v1manifest.ManifestTypeRoot], err = v1manifest.SignManifest(root, keys[v1manifest.ManifestTypeRoot]...)
	if err != nil {
		return err
	}

	// init snapshot
	snapshot, err = snapshot.SetVersions(signedManifests)
	if err != nil {
		return err
	}
	signedManifests[v1manifest.ManifestTypeSnapshot], err = v1manifest.SignManifest(snapshot, keys[v1manifest.ManifestTypeSnapshot]...)
	if err != nil {
		return err
	}

	timestamp, err = timestamp.SetSnapshot(signedManifests[v1manifest.ManifestTypeSnapshot])
	if err != nil {
		return errors.Trace(err)
	}

	signedManifests[v1manifest.ManifestTypeTimestamp], err = v1manifest.SignManifest(timestamp, keys[v1manifest.ManifestTypeTimestamp]...)
	if err != nil {
		return err
	}
	for _, m := range signedManifests {
		fname := filepath.Join(targetDir, m.Signed.Filename())
		switch m.Signed.Base().Ty {
		case v1manifest.ManifestTypeRoot:
			err := v1manifest.WriteManifestFile(FnameWithVersion(fname, m.Signed.Base().Version), m)
			if err != nil {
				return err
			}
			// A copy of the newest version which is 1.
			err = v1manifest.WriteManifestFile(fname, m)
			if err != nil {
				return err
			}
		case v1manifest.ManifestTypeComponent, v1manifest.ManifestTypeIndex:
			err := v1manifest.WriteManifestFile(FnameWithVersion(fname, m.Signed.Base().Version), m)
			if err != nil {
				return err
			}
		default:
			err = v1manifest.WriteManifestFile(fname, m)
			if err != nil {
				return err
			}
		}
	}

	return install.WriteLocalInstallScript(filepath.Join(targetDir, "local_install.sh"))
}

func cloneComponents(repo *V1Repository,
	components, selectedVersions []string,
	targetDir, tmpDir string,
	options CloneOptions) (map[string]*v1manifest.Component, error) {
	compManifests := map[string]*v1manifest.Component{}
	for _, name := range components {
		manifest, err := repo.FetchComponentManifest(name, true)
		if err != nil {
			return nil, errors.Annotatef(err, "fetch component '%s' manifest failed", name)
		}

		vs := combineVersions(options.Components[name], manifest, options.OSs, options.Archs, selectedVersions)
		var newManifest *v1manifest.Component
		if options.Full {
			newManifest = manifest
		} else {
			if len(vs) < 1 {
				continue
			}
			newManifest = &v1manifest.Component{
				SignedBase:  manifest.SignedBase,
				ID:          manifest.ID,
				Description: manifest.Description,
				Platforms:   map[string]map[string]v1manifest.VersionItem{},
			}
			// Include the nightly reference version
			if vs.Exist(version.NightlyVersion) {
				newManifest.Nightly = manifest.Nightly
				vs.Insert(manifest.Nightly)
			}
		}

		for _, goos := range options.OSs {
			for _, goarch := range options.Archs {
				platform := PlatformString(goos, goarch)
				versions := manifest.VersionListWithYanked(platform)
				if versions == nil {
					fmt.Printf("The component '%s' donesn't have %s/%s, skipped\n", name, goos, goarch)
				}
				for v, versionItem := range versions {
					if !options.Full {
						newVersions := newManifest.VersionListWithYanked(platform)
						if newVersions == nil {
							newVersions = map[string]v1manifest.VersionItem{}
							newManifest.Platforms[platform] = newVersions
						}
						newVersions[v] = versionItem
						if !checkVersion(options, vs, v) {
							versionItem.Yanked = true
							newVersions[v] = versionItem
							continue
						}
					}
					if versionItem.Yanked {
						continue
					}
					if err := download(targetDir, tmpDir, repo, &versionItem); err != nil {
						return nil, errors.Annotatef(err, "download resource: %s", name)
					}
				}
			}
		}
		compManifests[name] = newManifest
	}

	// Download TiUP binary
	for _, goos := range options.OSs {
		for _, goarch := range options.Archs {
			url := fmt.Sprintf("/tiup-%s-%s.tar.gz", goos, goarch)
			dstFile := filepath.Join(targetDir, url)
			tmpFile := filepath.Join(tmpDir, url)

			if err := repo.Mirror().Download(url, tmpDir); err != nil {
				if errors.Cause(err) == ErrNotFound {
					fmt.Printf("TiUP donesn't have %s/%s, skipped\n", goos, goarch)
					continue
				}
				return nil, err
			}
			// Move file to target directory if hashes pass verify.
			if err := os.Rename(tmpFile, dstFile); err != nil {
				return nil, err
			}
		}
	}

	return compManifests, nil
}

func download(targetDir, tmpDir string, repo *V1Repository, item *v1manifest.VersionItem) error {
	validate := func(dir string) error {
		hashes, n, err := ru.HashFile(path.Join(dir, item.URL))
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
		return nil
	}

	dstFile := filepath.Join(targetDir, item.URL)
	tmpFile := filepath.Join(tmpDir, item.URL)

	// Skip installed file if exists file valid
	if utils.IsExist(dstFile) {
		if err := validate(targetDir); err == nil {
			fmt.Println("Skip exists file:", filepath.Join(targetDir, item.URL))
			return nil
		}
	}

	err := repo.Mirror().Download(item.URL, tmpDir)
	if err != nil {
		return err
	}

	if err := validate(tmpDir); err != nil {
		return err
	}

	// Move file to target directory if hashes pass verify.
	return os.Rename(tmpFile, dstFile)
}

func checkVersion(options CloneOptions, versions set.StringSet, version string) bool {
	if options.Full || versions.Exist("all") || versions.Exist(version) {
		return true
	}
	// prefix match
	for v := range versions {

		if options.Prefix && strings.HasPrefix(version, v) {
			return true
		} else if version == v {
			return true
		}
	}
	return false
}

func combineVersions(versions *[]string, manifest *v1manifest.Component, oss, archs, selectedVersions []string) set.StringSet {
	if (versions == nil || len(*versions) < 1) && len(selectedVersions) < 1 {
		return nil
	}

	switch manifest.ID {
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

	// Some components version binding to TiDB
	coreSuites := set.NewStringSet("tidb", "tikv", "pd", "tiflash", "prometheus", "grafana", "ctl", "cdc")

	for _, os := range oss {
		for _, arch := range archs {
			platform := PlatformString(os, arch)
			versions := manifest.VersionList(platform)
			if versions == nil {
				continue
			}
			for _, selectedVersion := range selectedVersions {
				_, found := versions[selectedVersion]
				// Some TiUP components won't be bound version with TiDB, if cannot find
				// selected version we download the latest version to as a alternative
				if !found && !coreSuites.Exist(manifest.ID) {
					// Use the latest stable versionS if the selected version doesn't exist in specific platform
					var latest string
					for v := range versions {
						if v0manifest.Version(v).IsNightly() {
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
