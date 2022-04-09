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
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/utils"
	"golang.org/x/sync/errgroup"
)

const defaultJobs = 1

// CloneOptions represents the options of clone a remote mirror
type CloneOptions struct {
	Archs      []string
	OSs        []string
	Versions   []string
	Full       bool
	Components map[string]*[]string
	Prefix     bool
	Jobs       uint
}

// CloneMirror clones a local mirror from the remote repository
func CloneMirror(repo *V1Repository,
	components []string,
	tidbClusterVersionMapper func(string) string,
	targetDir string,
	selectedVersions []string,
	options CloneOptions) error {
	if strings.TrimRight(targetDir, "/") == strings.TrimRight(repo.Mirror().Source(), "/") {
		return errors.Errorf("Refusing to clone from %s to %s", targetDir, repo.Mirror().Source())
	}
	fmt.Printf("Start to clone mirror, targetDir is %s, source mirror is %s, selectedVersions are [%s]\n", targetDir, repo.Mirror().Source(), strings.Join(selectedVersions, ","))
	fmt.Println("If this does not meet expectations, please abort this process, read `tiup mirror clone --help` and run again")

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	// Temporary directory is used to save the unverified tarballs
	tmpDir := filepath.Join(targetDir, fmt.Sprintf("_tmp_%d", time.Now().UnixNano()))
	keyDir := filepath.Join(targetDir, "keys")

	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(keyDir, 0755); err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	fmt.Println("Arch", options.Archs)
	fmt.Println("OS", options.OSs)

	if len(options.OSs) == 0 || len(options.Archs) == 0 {
		return nil
	}

	var (
		initTime  = time.Now()
		expiresAt = initTime.Add(50 * 365 * 24 * time.Hour)
		root      = v1manifest.NewRoot(initTime)
		index     = v1manifest.NewIndex(initTime)
	)

	// All offline expires at 50 years to prevent manifests stale
	root.SetExpiresAt(expiresAt)
	index.SetExpiresAt(expiresAt)

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
	if _, err := v1manifest.SaveKeyInfo(ownerkeyInfo, "pingcap", keyDir); err != nil {
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
	snapshot.SetExpiresAt(expiresAt)

	componentManifests, err := cloneComponents(repo, components, selectedVersions, tidbClusterVersionMapper, targetDir, tmpDir, options)
	if err != nil {
		return err
	}

	for name, component := range componentManifests {
		component.SetExpiresAt(expiresAt)
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
	timestamp.SetExpiresAt(expiresAt)

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
	tidbClusterVersionMapper func(string) string,
	targetDir, tmpDir string,
	options CloneOptions) (map[string]*v1manifest.Component, error) {
	compManifests := map[string]*v1manifest.Component{}

	jobs := options.Jobs
	if jobs <= 0 {
		jobs = defaultJobs
	}
	errG := &errgroup.Group{}
	tickets := make(chan struct{}, jobs)
	defer func() { close(tickets) }()

	for _, name := range components {
		manifest, err := repo.FetchComponentManifest(name, true)
		if err != nil {
			return nil, errors.Annotatef(err, "fetch component '%s' manifest failed", name)
		}

		vs := combineVersions(options.Components[name], tidbClusterVersionMapper, manifest, options.OSs, options.Archs, selectedVersions)

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
			if vs.Exist(utils.NightlyVersionAlias) {
				newManifest.Nightly = manifest.Nightly
				vs.Insert(manifest.Nightly)
			}
		}

		platforms := []string{}
		for _, goos := range options.OSs {
			for _, goarch := range options.Archs {
				platforms = append(platforms, PlatformString(goos, goarch))
			}
		}
		if len(platforms) > 0 {
			platforms = append(platforms, v1manifest.AnyPlatform)
		}

		for _, platform := range platforms {
			for v, versionItem := range manifest.Platforms[platform] {
				if !options.Full {
					newVersions := newManifest.Platforms[platform]
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
				if _, err := repo.FetchComponentManifest(name, false); err != nil || versionItem.Yanked {
					// The component or the version is yanked, skip download binary
					continue
				}
				name, versionItem := name, versionItem
				errG.Go(func() error {
					tickets <- struct{}{}
					defer func() { <-tickets }()

					err := download(targetDir, tmpDir, repo, &versionItem)
					if err != nil {
						return errors.Annotatef(err, "download resource: %s", name)
					}
					return nil
				})
			}
		}
		compManifests[name] = newManifest
	}
	if err := errG.Wait(); err != nil {
		return nil, err
	}

	// Download TiUP binary
	for _, goos := range options.OSs {
		for _, goarch := range options.Archs {
			url := fmt.Sprintf("/tiup-%s-%s.tar.gz", goos, goarch)
			dstFile := filepath.Join(targetDir, url)
			tmpFile := filepath.Join(tmpDir, url)

			if err := repo.Mirror().Download(url, tmpDir); err != nil {
				if errors.Cause(err) == ErrNotFound {
					fmt.Printf("TiUP doesn't have %s/%s, skipped\n", goos, goarch)
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
			return err
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
			fmt.Println("Skipping existing file:", filepath.Join(targetDir, item.URL))
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

func combineVersions(versions *[]string,
	tidbClusterVersionMapper func(string) string,
	manifest *v1manifest.Component, oss, archs,
	selectedVersions []string) set.StringSet {
	if (versions == nil || len(*versions) < 1) && len(selectedVersions) < 1 {
		return nil
	}

	if bindver := tidbClusterVersionMapper(manifest.ID); bindver != "" {
		return set.NewStringSet(bindver)
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

			// set specified version with latest tag
			if result.Exist(utils.LatestVersionAlias) {
				latest := manifest.LatestVersion(platform)
				if latest != "" {
					result.Insert(latest)
				}
			}

			for _, selectedVersion := range selectedVersions {
				if selectedVersion == utils.NightlyVersionAlias {
					selectedVersion = manifest.Nightly
				}

				if selectedVersion == utils.LatestVersionAlias {
					latest := manifest.LatestVersion(platform)
					if latest == "" {
						continue
					}

					fmt.Printf("%s %s/%s found the lastest version %s\n", manifest.ID, os, arch, latest)
					// set latest version
					selectedVersion = latest
				}

				_, found := versions[selectedVersion]
				// Some TiUP components won't be bound version with TiDB, if cannot find
				// selected version we download the latest version to as a alternative
				if !found && !coreSuites.Exist(manifest.ID) {
					// Use the latest stable versionS if the selected version doesn't exist in specific platform
					latest := manifest.LatestVersion(platform)
					if latest == "" {
						continue
					}
					if selectedVersion != utils.LatestVersionAlias {
						fmt.Printf("%s %s/%s %s not found, using %s instead.\n", manifest.ID, os, arch, selectedVersion, latest)
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
