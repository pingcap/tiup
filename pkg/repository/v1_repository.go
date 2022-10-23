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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	cjson "github.com/gibson042/canonicaljson-go"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/localdata"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/pingcap/tiup/pkg/utils"
	"golang.org/x/mod/semver"
	"golang.org/x/sync/errgroup"
)

// ErrUnknownComponent represents the specific component cannot be found in index.json
var ErrUnknownComponent = errors.New("unknown component")

// ErrUnknownVersion represents the specific component version cannot be found in component.json
var ErrUnknownVersion = errors.New("unknown version")

// V1Repository represents a remote repository viewed with the v1 manifest design.
type V1Repository struct {
	Options
	mirror    Mirror
	local     v1manifest.LocalManifests
	timestamp *v1manifest.Manifest
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

// WithOptions clone a new V1Repository with given options
func (r *V1Repository) WithOptions(opts Options) *V1Repository {
	return NewV1Repo(r.Mirror(), opts, r.Local())
}

// Mirror returns Mirror
func (r *V1Repository) Mirror() Mirror {
	return r.mirror
}

// Local returns the local cached manifests
func (r *V1Repository) Local() v1manifest.LocalManifests {
	return r.local
}

// UpdateComponents updates the components described by specs.
func (r *V1Repository) UpdateComponents(specs []ComponentSpec) error {
	err := r.ensureManifests()
	if err != nil {
		return err
	}

	keepSource := false
	if v := os.Getenv(localdata.EnvNameKeepSourceTarget); v == "enable" || v == "true" {
		keepSource = true
	}
	var errs []string
	for _, spec := range specs {
		manifest, err := r.updateComponentManifest(spec.ID, false)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s, component: %s, mirror %s", err, spec.ID, r.local.Name()))
			if err == ErrUnknownComponent {
				continue
			}
		}

		if spec.Version == utils.NightlyVersionAlias {
			if !manifest.HasNightly(r.PlatformString()) {
				fmt.Printf("The component `%s` on platform %s does not have a nightly version; skipped\n", spec.ID, r.PlatformString())
				continue
			}

			spec.Version = manifest.Nightly
		}

		if spec.Version == "" {
			ver, _, err := r.LatestStableVersion(spec.ID, false)
			if err != nil {
				errs = append(errs, fmt.Sprintf("%s, component: %s, mirror %s", err, spec.ID, r.local.Name()))
				continue
			}
			spec.Version = ver.String()
		}
		if !spec.Force {
			installed, err := r.local.ComponentInstalled(spec.ID, spec.Version)
			if err != nil {
				return err
			}
			if installed {
				fmt.Printf("component %s version %s is already installed\n", spec.ID, spec.Version)
				continue
			}
		}

		targetDir := filepath.Join(r.local.TargetRootDir(), localdata.ComponentParentDir, r.local.Name(), spec.ID, spec.Version)
		if spec.TargetDir != "" {
			targetDir = spec.TargetDir
		}

		versionItem, err := r.ComponentVersion(spec.ID, spec.Version, false)
		if err != nil {
			return err
		}

		target := filepath.Join(targetDir, versionItem.URL)
		err = r.DownloadComponent(versionItem, target)
		if err != nil {
			os.RemoveAll(targetDir)
			errs = append(errs, fmt.Sprintf("%s, component: %s, mirror %s", err, spec.ID, r.local.Name()))
			continue
		}

		reader, err := os.Open(target)
		if err != nil {
			os.RemoveAll(targetDir)
			errs = append(errs, fmt.Sprintf("%s, component: %s, mirror %s", err, spec.ID, r.local.Name()))
			continue
		}

		err = r.local.InstallComponent(reader, targetDir, spec.ID, spec.Version, versionItem.URL, r.DisableDecompress)
		reader.Close()

		if err != nil {
			os.RemoveAll(targetDir)
			errs = append(errs, fmt.Sprintf("%s, component: %s, mirror %s", err, spec.ID, r.local.Name()))
		}

		// remove the source gzip target if expand is on && no keep source
		if !r.DisableDecompress && !keepSource {
			_ = os.Remove(target)
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "\n"))
	}

	return nil
}

// ensureManifests ensures that the snapshot, root, and index manifests are up to date and saved in r.local.
func (r *V1Repository) ensureManifests() error {
	defer func(start time.Time) {
		logprinter.Verbose("Ensure manifests finished in %s", time.Since(start))
	}(time.Now())

	// Update snapshot.
	snapshot, err := r.updateLocalSnapshot()
	if err != nil {
		return err
	}

	return r.updateLocalIndex(snapshot)
}

// Postcondition: if returned error is nil, then the local snapshot and timestamp are up to date and return the snapshot
func (r *V1Repository) updateLocalSnapshot() (*v1manifest.Snapshot, error) {
	defer func(start time.Time) {
		logprinter.Verbose("Update local snapshot finished in %s", time.Since(start))
	}(time.Now())

	timestampChanged, tsManifest, err := r.fetchTimestamp()
	if v1manifest.IsSignatureError(errors.Cause(err)) || v1manifest.IsExpirationError(errors.Cause(err)) {
		// The signature is wrong, update our signatures from the root manifest and try again.
		err = r.updateLocalRoot()
		if err != nil {
			return nil, err
		}
		timestampChanged, tsManifest, err = r.fetchTimestamp()
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	var snapshot v1manifest.Snapshot
	snapshotManifest, snapshotExists, err := r.local.LoadManifest(&snapshot)
	if err != nil {
		return nil, err
	}

	hash := tsManifest.Signed.(*v1manifest.Timestamp).SnapshotHash()
	bytes, err := cjson.Marshal(snapshotManifest)
	if err != nil {
		return nil, err
	}
	hash256 := sha256.Sum256(bytes)

	snapshotChanged := true
	// TODO: check changed in fetchTimestamp by compared to the raw local snapshot instead of timestamp.
	if snapshotExists && hash.Hashes[v1manifest.SHA256] == hex.EncodeToString(hash256[:]) {
		// Nothing has changed in snapshot.json
		snapshotChanged = false
	}

	if snapshotChanged {
		manifest, err := r.fetchManifestWithHash(v1manifest.ManifestURLSnapshot, &snapshot, &hash)
		if err != nil {
			return nil, err
		}

		// Persistent the snapshot first and prevent the snapshot.json/timestamp.json inconsistent
		// 1. timestamp.json is fetched every time
		// 2. when interrupted after timestamp.json been saved but snapshot.json have not, the snapshot.json is not going to be updated anymore
		err = r.local.SaveManifest(manifest, v1manifest.ManifestFilenameSnapshot)
		if err != nil {
			return nil, err
		}

		// Update root if needed and restart the update process
		var oldRoot v1manifest.Root
		_, _, err = r.local.LoadManifest(&oldRoot)
		if err != nil {
			return nil, err
		}
		newRootVersion := snapshot.Meta[v1manifest.ManifestURLRoot].Version

		if newRootVersion > oldRoot.Version {
			err := r.updateLocalRoot()
			if err != nil {
				return nil, err
			}
			return r.updateLocalSnapshot()
		}
	}

	if timestampChanged {
		err = r.local.SaveManifest(tsManifest, v1manifest.ManifestFilenameTimestamp)
		if err != nil {
			return nil, err
		}
	}

	return &snapshot, nil
}

// FnameWithVersion returns a filename, which contains the specific version number
func FnameWithVersion(fname string, version uint) string {
	base := filepath.Base(fname)
	dir := filepath.Dir(fname)

	versionBase := strconv.Itoa(int(version)) + "." + base
	return filepath.Join(dir, versionBase)
}

func (r *V1Repository) updateLocalRoot() error {
	defer func(start time.Time) {
		logprinter.Verbose("Update local root finished in %s", time.Since(start))
	}(time.Now())

	oldRoot, err := r.loadRoot()
	if err != nil {
		return err
	}
	startVersion := oldRoot.Version
	keyStore := *r.local.KeyStore()

	var newManifest *v1manifest.Manifest
	for {
		var newRoot v1manifest.Root
		url := FnameWithVersion(v1manifest.ManifestURLRoot, oldRoot.Version+1)
		nextManifest, err := r.fetchManifestWithKeyStore(url, &newRoot, maxRootSize, &keyStore)
		if err != nil {
			// Break if we have read the latest version.
			if errors.Cause(err) == ErrNotFound {
				break
			}
			return err
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
			return err
		}
		oldRoot = &newRoot
	}

	// We didn't change anything.
	if startVersion == oldRoot.Version {
		return nil
	}

	// Check expire of this version.
	err = v1manifest.CheckExpiry(v1manifest.ManifestFilenameRoot, oldRoot.Expires)
	if err != nil {
		return err
	}

	// Save the new trusted root without a version number. This action will also update the key store in r.local from
	// the new root.
	err = r.local.SaveManifest(newManifest, v1manifest.ManifestFilenameRoot)
	if err != nil {
		return err
	}

	return nil
}

// Precondition: the root manifest has been updated if necessary.
func (r *V1Repository) updateLocalIndex(snapshot *v1manifest.Snapshot) error {
	defer func(start time.Time) {
		logprinter.Verbose("Update local index finished in %s", time.Since(start))
	}(time.Now())

	// Update index (if needed).
	var oldIndex v1manifest.Index
	_, exists, err := r.local.LoadManifest(&oldIndex)
	if err != nil {
		return err
	}

	snapIndexVersion := snapshot.Meta[v1manifest.ManifestURLIndex].Version

	if exists && oldIndex.Version == snapIndexVersion {
		return nil
	}

	root, err := r.loadRoot()
	if err != nil {
		return err
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

	if exists && index.Version < oldIndex.Version {
		return errors.Errorf("index manifest has a version number < the old manifest (%v, %v)", index.Version, oldIndex.Version)
	}

	return r.local.SaveManifest(manifest, v1manifest.ManifestFilenameIndex)
}

// Precondition: the snapshot and index manifests exist and are up to date.
func (r *V1Repository) updateComponentManifest(id string, withYanked bool) (*v1manifest.Component, error) {
	defer func(start time.Time) {
		logprinter.Verbose("update component '%s' manifest finished in %s", id, time.Since(start))
	}(time.Now())

	// Find the component's entry in the index and snapshot manifests.
	var index v1manifest.Index
	_, _, err := r.local.LoadManifest(&index)
	if err != nil {
		return nil, err
	}
	var components map[string]v1manifest.ComponentItem
	if withYanked {
		components = index.ComponentListWithYanked()
	} else {
		components = index.ComponentList()
	}

	item, ok := components[id]
	if !ok {
		return nil, ErrUnknownComponent
	}
	var snapshot v1manifest.Snapshot
	_, _, err = r.local.LoadManifest(&snapshot)
	if err != nil {
		return nil, err
	}

	filename := v1manifest.ComponentManifestFilename(id)
	url, fileVersion, err := snapshot.VersionedURL(item.URL)
	if err != nil {
		return nil, err
	}

	oldVersion := r.local.ManifestVersion(filename)

	if oldVersion != 0 && oldVersion == fileVersion.Version {
		// We're up to date, load the old manifest from disk.
		comp, err := r.local.LoadComponentManifest(&item, filename)
		if comp == nil && err == nil {
			err = fmt.Errorf("component %s does not exist", id)
		}
		return comp, err
	}

	var component v1manifest.Component
	manifest, fetchErr := r.fetchComponentManifest(&item, url, &component, fileVersion.Length)
	if fetchErr != nil {
		// ignore manifest expiration error here and continue building component object,
		// the manifest expiration error should be handled by caller, so try to return it
		// with a valid component object.
		if !v1manifest.IsExpirationError(errors.Cause(fetchErr)) {
			return nil, fetchErr
		}
	}

	if oldVersion != 0 && component.Version < oldVersion {
		return nil, fmt.Errorf("component manifest for %s has a version number < the old manifest (%v, %v)", id, component.Version, oldVersion)
	}

	err = r.local.SaveComponentManifest(manifest, filename)
	if err != nil {
		return nil, err
	}

	return &component, fetchErr
}

// DownloadComponent downloads the component specified by item into local file,
// the component will be removed if hash is not correct
func (r *V1Repository) DownloadComponent(item *v1manifest.VersionItem, target string) error {
	targetDir := filepath.Dir(target)
	err := r.mirror.Download(item.URL, targetDir)
	if err != nil {
		return err
	}

	// the downloaded file is named by item.URL, which maybe differ to target name
	if downloaded := path.Join(targetDir, item.URL); downloaded != target {
		err := os.Rename(downloaded, target)
		if err != nil {
			return err
		}
	}

	reader, err := os.Open(target)
	if err != nil {
		return err
	}

	_, err = checkHash(reader, item.Hashes[v1manifest.SHA256])
	reader.Close()
	if err != nil {
		// remove the target compoonent to avoid attacking
		_ = os.Remove(target)
		return errors.Errorf("validation failed for %s: %s", target, err)
	}
	return nil
}

// PurgeTimestamp remove timestamp cache from repository
func (r *V1Repository) PurgeTimestamp() {
	r.timestamp = nil
}

// FetchTimestamp downloads the timestamp file, validates it, and checks if the snapshot hash in it
// has the same value of our local one. (not hashing the snapshot file itself)
// Return weather the manifest is changed compared to the one in local ts and the FileHash of snapshot.
func (r *V1Repository) fetchTimestamp() (changed bool, manifest *v1manifest.Manifest, err error) {
	// check cache first
	if r.timestamp != nil {
		return false, r.timestamp, nil
	}

	defer func(start time.Time) {
		logprinter.Verbose("Fetch timestamp finished in %s", time.Since(start))
		r.timestamp = manifest
	}(time.Now())

	var ts v1manifest.Timestamp
	manifest, err = r.fetchManifest(v1manifest.ManifestURLTimestamp, &ts, maxTimeStampSize)
	if err != nil {
		return false, nil, err
	}

	hash := ts.SnapshotHash()

	var localTs v1manifest.Timestamp
	_, exists, err := r.local.LoadManifest(&localTs)
	if err != nil {
		return false, nil, err
	}

	switch {
	case !exists:
		changed = true
	case ts.Version < localTs.Version:
		return false, nil, fmt.Errorf("timestamp manifest has a version number < the old manifest (%v, %v)", ts.Version, localTs.Version)
	case hash.Hashes[v1manifest.SHA256] != localTs.SnapshotHash().Hashes[v1manifest.SHA256]:
		changed = true
	}

	return changed, manifest, nil
}

// PlatformString returns a string identifying the current system.
func PlatformString(os, arch string) string {
	return fmt.Sprintf("%s/%s", os, arch)
}

// PlatformString returns a string identifying the current system.
func (r *V1Repository) PlatformString() string {
	return PlatformString(r.GOOS, r.GOARCH)
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
			return nil, errors.Annotatef(err, "validation failed for %s", url)
		}

		return v1manifest.ReadManifest(bufReader, role, r.local.KeyStore())
	})
}

func (r *V1Repository) fetchBase(url string, maxSize uint, f func(reader io.Reader) (*v1manifest.Manifest, error)) (*v1manifest.Manifest, error) {
	reader, err := r.mirror.Fetch(url, int64(maxSize))
	if err != nil {
		return nil, errors.Annotatef(err, "fetch %s from mirror(%s) failed", url, r.mirror.Source())
	}
	defer reader.Close()

	m, err := f(reader)
	if err != nil {
		return m, errors.Annotatef(err, "read manifest from mirror(%s) failed", r.mirror.Source())
	}
	return m, nil
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

func (r *V1Repository) loadRoot() (*v1manifest.Root, error) {
	root := new(v1manifest.Root)
	_, exists, err := r.local.LoadManifest(root)
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, errors.New("no trusted root in the local manifests")
	}
	return root, nil
}

// FetchRootManifest fetch the root manifest.
func (r *V1Repository) FetchRootManifest() (root *v1manifest.Root, err error) {
	err = r.ensureManifests()
	if err != nil {
		return nil, err
	}

	root = new(v1manifest.Root)
	_, exists, err := r.local.LoadManifest(root)
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, errors.Errorf("no root manifest")
	}

	return root, nil
}

// FetchIndexManifest fetch the index manifest.
func (r *V1Repository) FetchIndexManifest() (index *v1manifest.Index, err error) {
	err = r.ensureManifests()
	if err != nil {
		return nil, err
	}

	index = new(v1manifest.Index)
	_, exists, err := r.local.LoadManifest(index)
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, errors.Errorf("no index manifest")
	}

	return index, nil
}

// DownloadTiUP downloads the tiup tarball and expands it into targetDir
func (r *V1Repository) DownloadTiUP(targetDir string) error {
	var spec = ComponentSpec{
		TargetDir: targetDir,
		ID:        TiUPBinaryName,
		Version:   "",
		Force:     false,
	}
	return r.UpdateComponents([]ComponentSpec{spec})
}

// UpdateComponentManifests updates all components's manifest to the latest version
func (r *V1Repository) UpdateComponentManifests() error {
	index, err := r.FetchIndexManifest()
	if err != nil {
		return err
	}

	var g errgroup.Group
	for name := range index.Components {
		name := name
		g.Go(func() error {
			_, err := r.updateComponentManifest(name, false)
			if err != nil && errors.Cause(err) == ErrUnknownComponent {
				err = nil
			}
			return err
		})
	}
	err = g.Wait()
	return err
}

// GetComponentManifest fetch the component manifest.
func (r *V1Repository) GetComponentManifest(id string, withYanked bool) (com *v1manifest.Component, err error) {
	err = r.ensureManifests()
	if err != nil {
		return nil, err
	}

	return r.updateComponentManifest(id, withYanked)
}

// LocalComponentManifest load the component manifest from local.
func (r *V1Repository) LocalComponentManifest(id string, withYanked bool) (com *v1manifest.Component, err error) {
	index := v1manifest.Index{}
	_, exists, err := r.Local().LoadManifest(&index)
	if err == nil && exists {
		components := index.ComponentList()
		comp := components[id]
		filename := v1manifest.ComponentManifestFilename(id)
		componentManifest, err := r.Local().LoadComponentManifest(&comp, filename)
		if err == nil && componentManifest != nil {
			return componentManifest, nil
		}
	}
	return r.GetComponentManifest(id, withYanked)
}

// ComponentVersion returns version item of a component
func (r *V1Repository) ComponentVersion(id, ver string, includeYanked bool) (*v1manifest.VersionItem, error) {
	manifest, err := r.GetComponentManifest(id, includeYanked)
	if err != nil {
		return nil, err
	}
	if ver == utils.NightlyVersionAlias {
		if !manifest.HasNightly(r.PlatformString()) {
			return nil, errors.Annotatef(ErrUnknownVersion, "component %s does not have nightly on %s", id, r.PlatformString())
		}

		ver = manifest.Nightly
	}
	if ver == "" {
		v, _, err := r.LatestStableVersion(id, includeYanked)
		if err != nil {
			return nil, err
		}
		ver = v.String()
	}
	vi := manifest.VersionItem(r.PlatformString(), ver, includeYanked)
	if vi == nil {
		return nil, errors.Annotatef(ErrUnknownVersion, "version %s on %s for component %s not found", ver, r.PlatformString(), id)
	}
	return vi, nil
}

// LocalComponentVersion returns version item of a component from local manifest file
func (r *V1Repository) LocalComponentVersion(id, ver string, includeYanked bool) (*v1manifest.VersionItem, error) {
	manifest, err := r.LocalComponentManifest(id, includeYanked)
	if err != nil {
		return nil, err
	}

	if ver == utils.NightlyVersionAlias {
		if !manifest.HasNightly(r.PlatformString()) {
			return nil, errors.Annotatef(ErrUnknownVersion, "component %s does not have nightly on %s", id, r.PlatformString())
		}

		ver = manifest.Nightly
	}
	if ver == "" {
		v, _, err := r.LatestStableVersion(id, includeYanked)
		if err != nil {
			return nil, err
		}
		ver = v.String()
	}
	vi := manifest.VersionItem(r.PlatformString(), ver, includeYanked)
	if vi == nil {
		return nil, errors.Annotatef(ErrUnknownVersion, "version %s on %s for component %s not found", ver, r.PlatformString(), id)
	}
	return vi, nil
}

func findVersionFromManifest(id, constraint, platform string, manifest *v1manifest.Component) (string, error) {
	cons, err := utils.NewConstraint(constraint)
	if err != nil {
		return "", err
	}
	versions := manifest.VersionList(platform)
	verList := make([]string, 0, len(versions))
	for v := range versions {
		if v == manifest.Nightly {
			continue
		}
		verList = append(verList, v)
	}
	sort.Slice(verList, func(p, q int) bool {
		return semver.Compare(verList[p], verList[q]) > 0
	})
	for _, v := range verList {
		if cons.Check(v) {
			return v, nil
		}
	}
	return "", errors.Annotatef(ErrUnknownVersion, "version %s on %s for component %s not found", constraint, platform, id)
}

// ResolveComponentVersionWithPlatform resolves the latest version of a component that satisfies the constraint
func (r *V1Repository) ResolveComponentVersionWithPlatform(id, constraint, platform string) (utils.Version, error) {
	manifest, err := r.LocalComponentManifest(id, false)
	if err != nil {
		return "", err
	}
	var ver string
	switch constraint {
	case "", utils.LatestVersionAlias:
		v, _, err := r.LatestStableVersion(id, false)
		if err != nil {
			return "", err
		}
		ver = v.String()
	case utils.NightlyVersionAlias:
		v, _, err := r.LatestNightlyVersion(id)
		if err != nil {
			return "", err
		}
		ver = v.String()
	default:
		ver, err = findVersionFromManifest(id, constraint, platform, manifest)
		if err != nil {
			manifest, err = r.GetComponentManifest(id, false)
			if err != nil {
				return "", err
			}
			ver, err = findVersionFromManifest(id, constraint, platform, manifest)
			if err != nil {
				return "", errors.Annotatef(ErrUnknownVersion, "version %s on %s for component %s not found", constraint, platform, id)
			}
		}
	}
	return utils.Version(ver), nil
}

// ResolveComponentVersion resolves the latest version of a component that satisfies the constraint
func (r *V1Repository) ResolveComponentVersion(id, constraint string) (utils.Version, error) {
	return r.ResolveComponentVersionWithPlatform(id, constraint, r.PlatformString())
}

// LatestNightlyVersion returns the latest nightly version of specific component
func (r *V1Repository) LatestNightlyVersion(id string) (utils.Version, *v1manifest.VersionItem, error) {
	com, err := r.GetComponentManifest(id, false)
	if err != nil {
		return "", nil, err
	}

	if !com.HasNightly(r.PlatformString()) {
		return "", nil, fmt.Errorf("component %s doesn't have nightly version on platform %s", id, r.PlatformString())
	}

	return utils.Version(com.Nightly), com.VersionItem(r.PlatformString(), com.Nightly, false), nil
}

// LatestStableVersion returns the latest stable version of specific component
func (r *V1Repository) LatestStableVersion(id string, withYanked bool) (utils.Version, *v1manifest.VersionItem, error) {
	com, err := r.GetComponentManifest(id, withYanked)
	if err != nil {
		return "", nil, err
	}

	var versions map[string]v1manifest.VersionItem
	if withYanked {
		versions = com.VersionListWithYanked(r.PlatformString())
	} else {
		versions = com.VersionList(r.PlatformString())
	}
	if versions == nil {
		return "", nil, fmt.Errorf("component %s doesn't support platform %s", id, r.PlatformString())
	}

	var last string
	var lastStable string
	for v := range versions {
		if utils.Version(v).IsNightly() {
			continue
		}

		if last == "" || semver.Compare(last, v) < 0 {
			last = v
		}
		if semver.Prerelease(v) == "" && (lastStable == "" || semver.Compare(lastStable, v) < 0) {
			lastStable = v
		}
	}

	if lastStable == "" {
		if last == "" {
			return "", nil, fmt.Errorf("component %s doesn't has a stable version", id)
		}
		return utils.Version(last), com.VersionItem(r.PlatformString(), last, false), nil
	}

	return utils.Version(lastStable), com.VersionItem(r.PlatformString(), lastStable, false), nil
}

// BinaryPath return the binary path of the component.
// Support you have install the component, need to get entry from local manifest.
// Load the manifest locally only to get then Entry, do not force do something need access mirror.
func (r *V1Repository) BinaryPath(installPath string, componentID string, ver string) (string, error) {
	// We need yanked version because we may have installed that version before it was yanked
	versionItem, err := r.LocalComponentVersion(componentID, ver, true)
	if err != nil {
		return "", err
	}

	entry := versionItem.Entry
	if entry == "" {
		return "", errors.Errorf("cannot found entry for %s:%s", componentID, ver)
	}

	return filepath.Join(installPath, entry), nil
}
