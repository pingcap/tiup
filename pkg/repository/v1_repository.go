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
	"strconv"
	"strings"

	"golang.org/x/mod/semver"

	"github.com/pingcap-incubator/tiup/pkg/repository/v0manifest"
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
	// TargetDir it the target directory of the component,
	// Will use the default directory of Profile if it's empty.
	TargetDir string
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

// Mirror returns Mirror
func (r *V1Repository) Mirror() Mirror {
	return r.mirror
}

// UpdateComponents updates the components described by specs.
func (r *V1Repository) UpdateComponents(specs []ComponentSpec, nightly bool) error {
	_, err := r.ensureManifests()
	if err != nil {
		return errors.Trace(err)
	}

	var errs []string
	for _, spec := range specs {
		manifest, err := r.updateComponentManifest(spec.ID)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}

		if nightly {
			spec.Version = manifest.Nightly
		}

		if v0manifest.Version(spec.Version).IsNightly() && !manifest.HasNightly(r.PlatformString()) {
			errs = append(errs, fmt.Sprintf("the component `%s` does not have a nightly version; skipped", spec.ID))
			continue
		}

		platform := r.PlatformString()
		versions, ok := manifest.Platforms[platform]
		if !ok {
			errs = append(errs, fmt.Sprintf("platform %s not supported by component %s", platform, spec.ID))
			continue
		}

		version, versionItem, err := r.selectVersion(spec.ID, versions, spec.Version)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}

		if !spec.Force {
			installed, err := r.local.ComponentInstalled(spec.ID, version)
			if err != nil {
				return errors.Trace(err)
			}
			if installed {
				errs = append(errs, fmt.Sprintf("component %s version %s is already installed", spec.ID, version))
				continue
			}
		}

		reader, err := r.FetchComponent(versionItem)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}

		err = r.local.InstallComponent(reader, spec.TargetDir, spec.ID, version, versionItem.URL, r.DisableDecompress)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "\n"))
	}

	return nil
}

// ensureManifests ensures that the snapshot, root, and index manifests are up to date and saved in r.local.
// Returns true if the timestamp has changed,
func (r *V1Repository) ensureManifests() (bool, error) {
	// Load the root manifest from disk to populate the key store.
	var root v1manifest.Root
	_, _ = r.local.LoadManifest(&root)
	// We can ignore errors here since we'll try again later.

	// Update snapshot.
	snapshot, err := r.updateLocalSnapshot()
	if err != nil {
		return false, errors.Trace(err)
	}
	if snapshot == nil {
		return false, nil
	}

	// Update root.
	err = r.updateLocalRoot()
	if err != nil {
		return false, errors.Trace(err)
	}

	// Check that the version of root we have is the same as declared in the snapshot.
	newRoot, err := r.loadRoot()
	if err != nil {
		return false, errors.Trace(err)
	}
	snapRootVersion := snapshot.Meta[v1manifest.ManifestURLRoot].Version
	if newRoot.Version != snapRootVersion {
		return false, fmt.Errorf("root version mismatch. Expected: %v, found: %v", snapRootVersion, newRoot.Version)
	}

	// Update index (if needed).
	var index v1manifest.Index
	exists, err := r.local.LoadManifest(&index)
	if err != nil {
		return false, errors.Trace(err)
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
		var latest string
		var latestItem v1manifest.VersionItem
		for version, item := range versions {
			if v0manifest.Version(version).IsNightly() {
				continue
			}

			if latest == "" || semver.Compare(version, latest) > 0 {
				latest = version
				latestItem = item
			}
		}

		return latest, &latestItem, nil
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
			return nil, errors.Trace(err)
		}
		hash, err = r.checkTimestamp()
		if err != nil {
			return nil, errors.Trace(err)
		}
	} else if err != nil {
		return nil, errors.Trace(err)
	}
	if hash == nil {
		// Nothing has changed in the repo, return success.
		return nil, nil
	}

	var snapshot v1manifest.Snapshot
	manifest, err := r.fetchManifestWithHash(v1manifest.ManifestURLSnapshot, &snapshot, hash)
	if err != nil {
		return nil, errors.Trace(err)
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
	keyStore := *r.local.KeyStore()

	var newManifest *v1manifest.Manifest
	var newRoot v1manifest.Root
	for {
		url := fnameWithVersion(v1manifest.ManifestURLRoot, oldRoot.Version+1)
		nextManifest, err := r.fetchManifestWithKeyStore(url, &newRoot, maxRootSize, &keyStore)
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
			return errors.AddStack(err)
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

	// Save the new trusted root without a version number. This action will also update the key store in r.local from
	// the new root.
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
		return errors.Trace(err)
	}

	var snapshot v1manifest.Snapshot
	exists, err := r.local.LoadManifest(&snapshot)
	if err != nil {
		return errors.Trace(err)
	}
	if !exists {
		return errors.New("No snapshot")
	}

	url, fileVersion, err := snapshot.VersionedURL(root.Roles[v1manifest.ManifestTypeIndex].URL)
	if err != nil {
		return errors.Trace(err)
	}

	var index v1manifest.Index
	manifest, err := r.fetchManifest(url, &index, fileVersion.Length)
	if err != nil {
		return errors.Trace(err)
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
		return nil, errors.Trace(err)
	}
	item, ok := index.Components[id]
	if !ok {
		return nil, fmt.Errorf("unknown component: %s", id)
	}
	var snapshot v1manifest.Snapshot
	_, err = r.local.LoadManifest(&snapshot)
	if err != nil {
		return nil, errors.Trace(err)
	}

	filename := v1manifest.ComponentManifestFilename(id)
	url, fileVersion, err := snapshot.VersionedURL(item.URL)
	if err != nil {
		return nil, errors.Trace(err)
	}

	oldVersion := r.local.ManifestVersion(filename)

	if oldVersion != 0 && oldVersion == fileVersion.Version {
		// We're up to date, load the old manifest from disk.
		return r.local.LoadComponentManifest(&item, filename)
	}

	var component v1manifest.Component
	manifest, err := r.fetchComponentManifest(&item, url, &component, fileVersion.Length)
	if err != nil {
		return nil, errors.Trace(err)
	}

	if oldVersion != 0 && component.Version <= oldVersion {
		return nil, fmt.Errorf("component manifest for %s has a version number <= the old manifest (%v, %v)", id, component.Version, oldVersion)
	}

	err = r.local.SaveComponentManifest(manifest, filename)
	if err != nil {
		return nil, errors.Trace(err)
	}

	return &component, nil
}

// DownloadComponent downloads a component with specific version from repository
func (r *V1Repository) DownloadComponent(
	component string,
	version v0manifest.Version,
	versionItem *v1manifest.VersionItem,
) error {
	cr, err := r.FetchComponent(versionItem)
	if err != nil {
		return err
	}

	resName := fmt.Sprintf("%s-%s", component, version)

	filename := fmt.Sprintf("%s-%s-%s", resName, r.GOOS, r.GOARCH)
	return r.local.InstallComponent(cr, "", component, string(version), filename, r.DisableDecompress)
}

// FetchComponent downloads the component specified by item.
func (r *V1Repository) FetchComponent(item *v1manifest.VersionItem) (io.Reader, error) {
	reader, err := r.mirror.Fetch(item.URL, int64(item.Length))
	if err != nil {
		return nil, errors.Trace(err)
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
		return nil, errors.Trace(err)
	}
	hash := ts.SnapshotHash()

	var localTs v1manifest.Timestamp
	exists, err := r.local.LoadManifest(&localTs)
	if !exists {
		// We can't find a local timestamp, so we're going to have to update
		return &hash, r.local.SaveManifest(manifest, v1manifest.ManifestFilenameTimestamp)
	} else if err != nil {
		return nil, errors.Trace(err)
	}
	if hash.Hashes[v1manifest.SHA256] == localTs.SnapshotHash().Hashes[v1manifest.SHA256] {
		return nil, nil
	}

	if exists && ts.Version <= localTs.Version {
		return nil, fmt.Errorf("timestamp manifest has a version number <= the old manifest (%v, %v)", ts.Version, localTs.Version)
	}

	return &hash, r.local.SaveManifest(manifest, v1manifest.ManifestFilenameTimestamp)
}

// PlatformString returns a string identifying the current system.
func (r *V1Repository) PlatformString() string {
	return fmt.Sprintf("%s/%s", r.GOOS, r.GOARCH)
}

func (r *V1Repository) fetchComponentManifest(item *v1manifest.ComponentItem, url string, com *v1manifest.Component, maxSize uint) (*v1manifest.Manifest, error) {
	return r.fetchBase(url, maxSize, func(reader io.Reader) (*v1manifest.Manifest, error) {
		return v1manifest.ReadComponentManifest(reader, com, item, r.local.KeyStore())
	})
}

// fetchManifest downloads and validates a manifest from this repo.
func (r *V1Repository) fetchManifest(url string, role v1manifest.ValidManifest, maxSize uint) (*v1manifest.Manifest, error) {
	return r.fetchBase(url, maxSize, func(reader io.Reader) (*v1manifest.Manifest, error) {
		return v1manifest.ReadManifest(reader, role, r.local.KeyStore())
	})
}

func (r *V1Repository) fetchManifestWithKeyStore(url string, role v1manifest.ValidManifest, maxSize uint, keys *v1manifest.KeyStore) (*v1manifest.Manifest, error) {
	return r.fetchBase(url, maxSize, func(reader io.Reader) (*v1manifest.Manifest, error) {
		return v1manifest.ReadManifest(reader, role, keys)
	})
}

func (r *V1Repository) fetchManifestWithHash(url string, role v1manifest.ValidManifest, hash *v1manifest.FileHash) (*v1manifest.Manifest, error) {
	return r.fetchBase(url, hash.Length, func(reader io.Reader) (*v1manifest.Manifest, error) {
		bufReader, err := checkHash(reader, hash.Hashes[v1manifest.SHA256])
		if err != nil {
			return nil, errors.Trace(err)
		}

		return v1manifest.ReadManifest(bufReader, role, r.local.KeyStore())
	})
}

func (r *V1Repository) fetchBase(url string, maxSize uint, f func(reader io.Reader) (*v1manifest.Manifest, error)) (*v1manifest.Manifest, error) {
	reader, err := r.mirror.Fetch(url, int64(maxSize))
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer reader.Close()

	return f(reader)
}

func checkHash(reader io.Reader, sha256 string) (io.Reader, error) {
	buffer := new(bytes.Buffer)
	_, err := io.Copy(buffer, reader)
	if err != nil {
		return nil, errors.Trace(err)
	}

	b := buffer.Bytes()
	bufReader := bytes.NewReader(b)
	if err = utils.CheckSHA256(bufReader, sha256); err != nil {
		return nil, errors.Trace(err)
	}

	_, err = bufReader.Seek(0, io.SeekStart)
	if err != nil {
		return nil, errors.Trace(err)
	}

	return bufReader, nil
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

// FetchIndexManifest fetch the index manifest.
func (r *V1Repository) FetchIndexManifest() (index *v1manifest.Index, err error) {
	_, err = r.ensureManifests()
	if err != nil {
		return nil, errors.AddStack(err)
	}

	index = new(v1manifest.Index)
	exists, err := r.local.LoadManifest(index)
	if err != nil {
		return nil, errors.AddStack(err)
	}

	if !exists {
		return nil, errors.Errorf("no index manifest")
	}

	return index, nil
}

// DownloadTiup downloads the tiup tarball and expands it into targetDir
func (r *V1Repository) DownloadTiup(targetDir string) error {
	var spec = ComponentSpec{
		TargetDir: targetDir,
		ID:        "tiup",
		Version:   "",
		Force:     false,
	}
	return r.UpdateComponents([]ComponentSpec{spec}, false)
}

// FetchComponentManifest fetch the component manifest.
func (r *V1Repository) FetchComponentManifest(id string) (com *v1manifest.Component, err error) {
	_, err = r.ensureManifests()
	if err != nil {
		return nil, errors.AddStack(err)
	}

	return r.updateComponentManifest(id)
}

// ComponentVersion returns version item of a component
func (r *V1Repository) ComponentVersion(id, version string) (*v1manifest.VersionItem, error) {
	manifest, err := r.FetchComponentManifest(id)
	if err != nil {
		return nil, err
	}
	vi := manifest.VersionItem(r.PlatformString(), version)
	if vi == nil {
		return nil, fmt.Errorf("version %s on %s for component %s not found", version, r.PlatformString(), id)
	}
	return vi, nil
}

// BinaryPath return the binary path of the component.
// Support you have install the component, need to get entry from local manifest.
// Load the manifest locally only to get then Entry, do not force do something need access mirror.
func (r *V1Repository) BinaryPath(installPath string, componentID string, version string) (string, error) {
	var index v1manifest.Index
	_, err := r.local.LoadManifest(&index)
	if err != nil {
		return "", err
	}

	filename := v1manifest.ComponentManifestFilename(componentID)

	item := index.Components[componentID]
	component, err := r.local.LoadComponentManifest(&item, filename)
	if err != nil {
		return "", err
	}

	versionItem, ok := component.Platforms[r.PlatformString()][version]
	if !ok {
		return "", errors.Errorf("no version: %s", version)
	}

	entry := versionItem.Entry
	if entry == "" {
		return "", errors.Errorf("cannot found entry for %s:%s", componentID, version)
	}

	return filepath.Join(installPath, entry), nil
}
