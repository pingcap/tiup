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
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"

	"github.com/pingcap-incubator/tiup/pkg/repository/v1manifest"
	"github.com/pingcap-incubator/tiup/pkg/utils"
	"github.com/pingcap/errors"
)

// V1Repository represents a remote repository viewed with the v1 manifest design.
type V1Repository struct {
	Options
	mirror Mirror
	local  v1manifest.LocalManifests
}

// ComponentSpec describes a component a user would like to have or use.
type ComponentSpec struct {
	// ID is the id of the component
	ID string
	// Version describes the versions which are desirable; "" = use the most recent, compatible version.
	Version string
	// Force is true means overwrite any existing installation.
	Force bool
}

// NewV1Repo creates a new v1 repository from the given mirror
// local must exists a trusted root.
func NewV1Repo(mirror Mirror, opts Options, local v1manifest.LocalManifests) *V1Repository {
	if opts.GOOS == "" {
		opts.GOOS = runtime.GOOS
	}
	if opts.GOARCH == "" {
		opts.GOARCH = runtime.GOARCH
	}

	repo := &V1Repository{
		Options: opts,
		mirror:  mirror,
		local:   local,
	}

	return repo
}

const maxTimeStampSize uint = 1024
const maxRootSize uint = 1024 * 1024

// UpdateComponents updates the components described by specs.
func (r *V1Repository) UpdateComponents(specs []ComponentSpec) error {
	_, err := r.ensureManifests()
	if err != nil {
		return err
	}
	for _, spec := range specs {
		manifest, err := r.updateComponentManifest(spec.ID)
		if err != nil {
			// TODO could report error and continue
			return err
		}

		platform := utils.PlatformString()
		versions, ok := manifest.Platforms[platform]
		if !ok {
			// TODO could report error and continue
			return fmt.Errorf("platform %s not supported by component %s", platform, spec.ID)
		}

		version, versionItem, err := r.selectVersion(spec.ID, versions, spec.Version)
		if err != nil {
			// TODO could report error and continue
			return err
		}

		if !spec.Force {
			installed, err := r.local.ComponentInstalled(spec.ID, version)
			if err != nil {
				return err
			}
			if installed {
				// TODO could report error and continue
				return fmt.Errorf("component %s version %s is already installed", spec.ID, version)
			}
		}

		reader, err := r.downloadComponent(versionItem)
		if err != nil {
			// TODO could report error and continue
			return err
		}

		err = r.local.InstallComponent(reader, spec.ID, version)
		if err != nil {
			// TODO could report error and continue
			return err
		}
	}
	return nil
}

// ensureManifests ensures that the snapshot, root, and index manifests are up to date and saved in r.local.
// Returns true if the timestamp has changed,
func (r *V1Repository) ensureManifests() (bool, error) {
	// Update snapshot.
	snapshot, err := r.updateLocalSnapshot()
	if err != nil {
		return false, err
	}
	if snapshot == nil {
		return false, nil
	}

	// Update root.
	err = r.updateLocalRoot()
	if err != nil {
		return false, err
	}

	// Check that the version of root we have is the same as declared in the snapshot.
	root, err := r.loadRoot()
	if err != nil {
		return false, err
	}
	snapRootVersion := snapshot.Meta[v1manifest.ManifestURLRoot].Version
	if root.Version != snapRootVersion {
		return false, fmt.Errorf("root version mismatch. Expected: %v, found: %v", snapRootVersion, root.Version)
	}

	// Update index (if needed).
	var index v1manifest.Index
	exists, err := r.local.LoadManifest(&index)
	if err != nil {
		return false, err
	}
	snapIndexVersion := snapshot.Meta[v1manifest.ManifestURLIndex].Version
	if exists && index.Version == snapIndexVersion {
		return true, nil
	}

	return true, r.updateLocalIndex()
}

func (r *V1Repository) selectVersion(id string, versions map[string]v1manifest.VersionItem, target string) (string, *v1manifest.VersionItem, error) {
	// TODO we should check what version the user has currently installed and only update to the same semver major version unless they force upgrade.

	if target == "" {
		versionSlice := make([]string, 0, len(versions))
		for v := range versions {
			versionSlice = append(versionSlice, v)
		}
		sort.Strings(versionSlice)
		// TODO I think it is not actually correct semver order because versions can have a text suffix.
		version := versionSlice[len(versionSlice)-1]
		item := versions[version]
		return version, &item, nil
	}

	item, ok := versions[target]
	if !ok {
		// TODO we should return a semver-compatible version if one exists.
		return "", nil, fmt.Errorf("version %s not supported by component %s", target, id)
	}
	return target, &item, nil
}

// Postcondition: if returned error is nil, then the local snapshot and timestamp are up to date.
// Returns nil if the timestamp has not changed, or the new snapshot if it has.
func (r *V1Repository) updateLocalSnapshot() (*v1manifest.Snapshot, error) {
	hash, err := r.checkTimestamp()
	if v1manifest.IsSignatureError(errors.Cause(err)) {
		// The signature is wrong, update our signatures from the root manifest and try again.
		err = r.updateLocalRoot()
		if err != nil {
			return nil, err
		}
		hash, err = r.checkTimestamp()
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	if hash == nil {
		// Nothing has changed in the repo, return success.
		return nil, nil
	}

	var snapshot v1manifest.Snapshot
	manifest, err := r.fetchManifestWithHash(v1manifest.ManifestURLSnapshot, &snapshot, hash)
	if err != nil {
		return nil, err
	}

	return &snapshot, r.local.SaveManifest(manifest, v1manifest.ManifestFilenameSnapshot)
}

func fnameWithVersion(fname string, version uint) string {
	base := filepath.Base(fname)
	dir := filepath.Dir(fname)

	versionBase := strconv.Itoa(int(version)) + "." + base
	return filepath.Join(dir, versionBase)
}

func (r *V1Repository) updateLocalRoot() error {
	oldRoot, err := r.loadRoot()
	if err != nil {
		return errors.AddStack(err)
	}
	startVersion := oldRoot.Version

	var newManifest *v1manifest.Manifest
	var newRoot v1manifest.Root
	for {
		url := fnameWithVersion(v1manifest.ManifestURLRoot, oldRoot.Version+1)
		nextManifest, err := r.fetchManifestWithRoot(url, &newRoot, maxRootSize, oldRoot)
		if err != nil {
			// Break if we have read the newest version.
			if errors.Cause(err) == ErrNotFound {
				break
			}
			return errors.AddStack(err)
		}
		newManifest = nextManifest

		if newRoot.Version != oldRoot.Version+1 {
			return errors.Errorf("root version is %d, but should be: %d", newRoot.Version, oldRoot.Version+1)
		}

		if err = v1manifest.ExpiresAfter(&newRoot, oldRoot); err != nil {
			return err
		}

		// This is a valid new version.
		err = r.local.SaveManifest(newManifest, v1manifest.RootManifestFilename(newRoot.Version))
		if err != nil {
			return errors.AddStack(err)
		}
		oldRoot = &newRoot
	}

	// We didn't change anything.
	if startVersion == oldRoot.Version {
		return nil
	}

	// Check expire of this version.
	err = v1manifest.CheckExpiry(oldRoot.Expires)
	if err != nil {
		return errors.AddStack(err)
	}

	// Save the new trusted root without a version number.
	err = r.local.SaveManifest(newManifest, v1manifest.ManifestFilenameRoot)
	if err != nil {
		return errors.AddStack(err)
	}

	return nil
}

// Precondition: the index manifest actually requires updating, snapshot manifest exists, and the root manifest has been updated if necessary.
func (r *V1Repository) updateLocalIndex() error {
	root, err := r.loadRoot()
	if err != nil {
		return err
	}

	var snapshot v1manifest.Snapshot
	exists, err := r.local.LoadManifest(&snapshot)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("No snapshot")
	}

	url, fileVersion, err := snapshot.VersionedURL(root.Roles[v1manifest.ManifestTypeIndex].URL)
	if err != nil {
		return err
	}

	var index v1manifest.Index
	manifest, err := r.fetchManifest(url, &index, fileVersion.Length)
	if err != nil {
		return err
	}

	// Check version number against old manifest
	var oldIndex v1manifest.Index
	exists, err = r.local.LoadManifest(&oldIndex)
	if exists {
		if err != nil {
			return err
		}
		if index.Version <= oldIndex.Version {
			return fmt.Errorf("index manifest has a version number <= the old manifest (%v, %v)", index.Version, oldIndex.Version)
		}
	}

	return r.local.SaveManifest(manifest, v1manifest.ManifestFilenameIndex)
}

// Precondition: the snapshot and index manifests exist and are up to date.
func (r *V1Repository) updateComponentManifest(id string) (*v1manifest.Component, error) {
	// Find the component's entry in the index and snapshot manifests.
	var index v1manifest.Index
	_, err := r.local.LoadManifest(&index)
	if err != nil {
		return nil, err
	}
	item, ok := index.Components[id]
	if !ok {
		return nil, fmt.Errorf("unknown component: %s", id)
	}
	var snapshot v1manifest.Snapshot
	_, err = r.local.LoadManifest(&snapshot)
	if err != nil {
		return nil, err
	}

	filename := v1manifest.ComponentManifestFilename(id)
	url, fileVersion, err := snapshot.VersionedURL(item.URL)
	if err != nil {
		return nil, err
	}

	oldManifest, err := r.local.LoadComponentManifest(filename)
	if err != nil {
		return nil, err
	}
	if oldManifest != nil && oldManifest.Version == fileVersion.Version {
		// We're up to date.
		return oldManifest, nil
	}

	var component v1manifest.Component
	manifest, err := r.fetchManifest(url, &component, fileVersion.Length)
	if err != nil {
		return nil, err
	}

	if oldManifest != nil && component.Version <= oldManifest.Version {
		return nil, fmt.Errorf("component manifest for %s has a version number <= the old manifest (%v, %v)", id, component.Version, oldManifest.Version)
	}

	err = r.local.SaveComponentManifest(manifest, filename)
	if err != nil {
		return nil, err
	}

	return &component, nil
}

// downloadComponent downloads the component specified by item.
func (r *V1Repository) downloadComponent(item *v1manifest.VersionItem) (io.Reader, error) {
	reader, err := r.mirror.Fetch(item.URL, int64(item.Length))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return checkHash(reader, item.Hashes[v1manifest.SHA256])
}

// CheckTimestamp downloads the timestamp file, validates it, and checks if the snapshot hash matches our local one.
// If they match, then there is nothing to update and we return nil. If they do not match, we return the
// snapshot's file info.
func (r *V1Repository) checkTimestamp() (*v1manifest.FileHash, error) {
	var ts v1manifest.Timestamp
	manifest, err := r.fetchManifest(v1manifest.ManifestURLTimestamp, &ts, maxTimeStampSize)
	if err != nil {
		return nil, err
	}
	hash := ts.SnapshotHash()

	var localTs v1manifest.Timestamp
	exists, err := r.local.LoadManifest(&localTs)
	if !exists {
		// We can't find a local timestamp, so we're going to have to update
		return &hash, r.local.SaveManifest(manifest, v1manifest.ManifestFilenameTimestamp)
	} else if err != nil {
		return nil, err
	}
	if hash.Hashes[v1manifest.SHA256] == localTs.SnapshotHash().Hashes[v1manifest.SHA256] {
		return nil, nil
	}

	if exists && ts.Version <= localTs.Version {
		return nil, fmt.Errorf("timestamp manifest has a version number <= the old manifest (%v, %v)", ts.Version, localTs.Version)
	}

	return &hash, r.local.SaveManifest(manifest, v1manifest.ManifestFilenameTimestamp)
}

// fetchManifest downloads and validates a manifest from this repo.
func (r *V1Repository) fetchManifest(url string, role v1manifest.ValidManifest, maxSize uint) (*v1manifest.Manifest, error) {
	root, err := r.loadRoot()
	if err != nil {
		return nil, err
	}

	return r.fetchBase(url, maxSize, func(reader io.Reader) (*v1manifest.Manifest, error) {
		return v1manifest.ReadManifest(reader, role, root)
	})
}

func (r *V1Repository) fetchManifestWithRoot(url string, role v1manifest.ValidManifest, maxSize uint, root *v1manifest.Root) (*v1manifest.Manifest, error) {
	return r.fetchBase(url, maxSize, func(reader io.Reader) (*v1manifest.Manifest, error) {
		return v1manifest.ReadManifest(reader, role, root)
	})
}

func (r *V1Repository) fetchManifestWithHash(url string, role v1manifest.ValidManifest, hash *v1manifest.FileHash) (*v1manifest.Manifest, error) {
	root, err := r.loadRoot()
	if err != nil {
		return nil, err
	}

	return r.fetchBase(url, hash.Length, func(reader io.Reader) (*v1manifest.Manifest, error) {
		bufReader, err := checkHash(reader, hash.Hashes[v1manifest.SHA256])
		if err != nil {
			return nil, err
		}

		return v1manifest.ReadManifest(bufReader, role, root)
	})
}

func checkHash(reader io.Reader, sha256 string) (io.Reader, error) {
	buffer := new(bytes.Buffer)
	_, err := io.Copy(buffer, reader)
	if err != nil {
		return nil, err
	}

	b := buffer.Bytes()
	bufReader := bytes.NewReader(b)
	if err = utils.CheckSHA256(bufReader, sha256); err != nil {
		return nil, err
	}

	_, err = bufReader.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}

	return bufReader, nil
}

func (r *V1Repository) fetchBase(url string, maxSize uint, f func(reader io.Reader) (*v1manifest.Manifest, error)) (*v1manifest.Manifest, error) {
	reader, err := r.mirror.Fetch(url, int64(maxSize))
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer reader.Close()

	return f(reader)
}

func (r *V1Repository) loadRoot() (*v1manifest.Root, error) {
	root := new(v1manifest.Root)
	exists, err := r.local.LoadManifest(root)
	if err != nil {
		return nil, errors.AddStack(err)
	}

	if !exists {
		return nil, errors.New("no trusted root in the local manifest")
	}
	return root, nil
}
