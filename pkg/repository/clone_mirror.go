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
	"path/filepath"
	"strings"
	"time"

	"github.com/pingcap-incubator/tiup/pkg/assets"

	cjson "github.com/gibson042/canonicaljson-go"
	"github.com/pingcap-incubator/tiup/pkg/repository/v1manifest"
	"github.com/pingcap-incubator/tiup/pkg/set"
	"github.com/pingcap-incubator/tiup/pkg/utils"
	"github.com/pingcap-incubator/tiup/pkg/version"
	"github.com/pingcap/errors"
	"golang.org/x/mod/semver"
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
func CloneMirror(repo *V1Repository, components []string, targetDir string, selectedVersions []string, options CloneOptions) error {
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
		if err := v1manifest.GenAndSaveKeys(keys, ty, int(v1manifest.ManifestsConfig[ty].Threshold), tmpDir); err != nil {
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
	if err := v1manifest.SaveKeyInfo(ownerkeyInfo, "pingcap", tmpDir); err != nil {
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

	const limitLength = 1024 * 1024

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
		bytes, err := cjson.Marshal(signedManifests[component.ID])
		if err != nil {
			return err
		}
		var _ = len(bytes) // this length is the not final length, since we still change the manifests before write it to disk.
		snapshot.Meta["/"+name] = v1manifest.FileVersion{
			Version: 1,
			Length:  limitLength,
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

	hash, _, err := HashManifest(signedManifests[v1manifest.ManifestTypeSnapshot])
	if err != nil {
		return errors.Trace(err)
	}
	timestamp.Meta = map[string]v1manifest.FileHash{
		v1manifest.ManifestURLSnapshot: {
			Hashes: hash,
			Length: limitLength,
		},
	}
	signedManifests[v1manifest.ManifestTypeTimestamp], err = v1manifest.SignManifest(timestamp, keys[v1manifest.ManifestTypeTimestamp]...)
	if err != nil {
		return err
	}
	for _, m := range signedManifests {
		fname := filepath.Join(tmpDir, m.Signed.Filename())
		switch m.Signed.Base().Ty {
		case v1manifest.ManifestTypeRoot:
			err := v1manifest.WriteManifestFile(FnameWithVersion(fname, 1), m)
			if err != nil {
				return err
			}
			// A copy of the newest version which is 1.
			err = v1manifest.WriteManifestFile(fname, m)
			if err != nil {
				return err
			}
		case v1manifest.ManifestTypeComponent, v1manifest.ManifestTypeIndex:
			err := v1manifest.WriteManifestFile(FnameWithVersion(fname, 1), m)
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

	return assets.WriteLocalInstallScript(filepath.Join(targetDir, "local_install.sh"))
}

func cloneComponents(repo *V1Repository,
	components, selectedVersions []string,
	targetDir, tmpDir string,
	options CloneOptions) (map[string]*v1manifest.Component, error) {
	compManifests := map[string]*v1manifest.Component{}
	for _, name := range components {
		manifest, err := repo.FetchComponentManifest(name)
		if err != nil {
			return nil, errors.Annotatef(err, "fetch component '%s' manifest", name)
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
				versions, found := manifest.Platforms[platform]
				if !found {
					fmt.Printf("The component '%s' donesn't %s/%s, skipped\n", name, goos, goarch)
				}
				for v, versionItem := range versions {
					if !checkVersion(options, vs, v) {
						continue
					}
					if !options.Full {
						newVersions, found := newManifest.Platforms[platform]
						if !found {
							newVersions = map[string]v1manifest.VersionItem{}
							newManifest.Platforms[platform] = newVersions
						}
						newVersions[v] = versionItem
					}
					if err := download(targetDir, tmpDir, repo, &versionItem); err != nil {
						return nil, errors.Annotatef(err, "download resource: %s", name)
					}
				}
			}
		}
		compManifests[name] = newManifest
	}

	return compManifests, nil
}

func download(targetDir, tmpDir string, repo *V1Repository, item *v1manifest.VersionItem) error {
	err := repo.Mirror().Download(item.URL, tmpDir)
	if err != nil {
		return err
	}
	hashes, n, err := HashFile(tmpDir, item.URL)
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

	for _, os := range oss {
		for _, arch := range archs {
			platform := PlatformString(os, arch)
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
