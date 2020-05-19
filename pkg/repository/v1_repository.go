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
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/pingcap-incubator/tiup/pkg/repository/crypto"
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

// NewV1Repo creates a new v1 repository from the given mirror
func NewV1Repo(mirror Mirror, opts Options, local v1manifest.LocalManifests) *V1Repository {
	if opts.GOOS == "" {
		opts.GOOS = runtime.GOOS
	}
	if opts.GOARCH == "" {
		opts.GOARCH = runtime.GOARCH
	}

	return &V1Repository{
		Options: opts,
		mirror:  mirror,
		local:   local,
	}
}

const maxTimeStampSize uint = 1024
const maxRootSize uint = 1024 * 1024

// If the snapshot has been updated, we return the new snapshot, if not we return nil.
// Postcondition: if returned error is nil, then the local snapshot and timestamp are up to date.
func (r *V1Repository) updateLocalSnapshot() (*v1manifest.Snapshot, error) {
	hash, err := r.checkTimestamp()
	if _, ok := err.(*v1manifest.SignatureError); ok {
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
	manifest, err := r.fetchManifest(v1manifest.ManifestURLSnapshot, &snapshot, r.local.Keys(), hash.Length)
	if err != nil {
		return nil, err
	}

	// TODO validate snapshot against hash

	err = r.local.SaveManifest(manifest, v1manifest.ManifestFilenameSnapshot)
	if err != nil {
		return nil, err
	}

	return &snapshot, nil
}

func fnameWithVersion(fname string, version uint) string {
	base := filepath.Base(fname)
	dir := filepath.Dir(fname)

	versionBase := strconv.Itoa(int(version)) + "." + base
	return filepath.Join(dir, versionBase)
}

func (r *V1Repository) updateLocalRoot() error {
	var root1 v1manifest.Root
	_, err := r.local.LoadManifest(&root1)
	if err != nil {
		return err
	}

	var newRoots []v1manifest.Manifest

	for version := root1.Version + 1; ; version++ {
		var root2 v1manifest.Root
		fname := fnameWithVersion(v1manifest.ManifestURLRoot, version)

		reader, err := r.mirror.Fetch(fname, int64(maxRootSize))
		if err != nil {
			// Break if we have read the newest version.
			if errors.Cause(err) == ErrNotFound {
				break
			}
			return errors.AddStack(err)
		}

		decoder := json.NewDecoder(reader)
		var m v1manifest.Manifest
		m.Signed = &root2
		err = decoder.Decode(&m)
		if err != nil {
			return err
		}

		if root2.Version != root1.Version+1 {
			return errors.Errorf("version is %d, but should be: %d", root2.Version, root1.Version+1)
		}

		// check signed by old version root
		threshold := root1.Roles[v1manifest.ManifestTypeRoot].Threshold
		keyStore, err := root1.GetRootKeyStore()
		if err != nil {
			return errors.AddStack(err)
		}

		err = m.VerifySignature(threshold, keyStore)
		if err != nil {
			return errors.AddStack(err)
		}

		// check signed by new version root
		threshold = root2.Roles[v1manifest.ManifestTypeRoot].Threshold
		keyStore, err = root2.GetRootKeyStore()
		if err != nil {
			return errors.AddStack(err)
		}

		err = m.VerifySignature(threshold, keyStore)
		if err != nil {
			return errors.AddStack(err)
		}

		// This is valid new version
		newRoots = append(newRoots, m)
		root1 = root2
	}

	if len(newRoots) == 0 {
		return nil
	}

	newTrusted := newRoots[len(newRoots)-1]
	// check expire of this version
	err = v1manifest.CheckExpire(newTrusted.Signed.Base().Expires)
	if err != nil {
		return errors.AddStack(err)
	}

	// Save the new trusted root.
	err = r.local.SaveManifest(&newTrusted, v1manifest.ManifestFilenameRoot)
	if err != nil {
		return errors.AddStack(err)
	}

	for _, m := range newRoots {
		filename := fnameWithVersion(v1manifest.ManifestTypeRoot, m.Signed.Base().Version)
		err = r.local.SaveManifest(&m, filename)
		if err != nil {
			return errors.AddStack(err)
		}
	}

	return nil
}

// Precondition: the index manifest actually requires updating, the root manifest has been updated if necessary.
func (r *V1Repository) updateLocalIndex(length uint) (*v1manifest.Index, error) {
	var root v1manifest.Root
	_, err := r.local.LoadManifest(&root)
	if err != nil {
		return nil, err
	}

	var snapshot v1manifest.Snapshot
	_, err = r.local.LoadManifest(&snapshot)
	if err != nil {
		return nil, err
	}

	url, err := snapshot.VersionedURL(root.Roles[v1manifest.ManifestTypeIndex].URL)
	if err != nil {
		return nil, err
	}

	var index v1manifest.Index
	manifest, err := r.fetchManifest(url, &index, r.local.Keys(), length)
	if err != nil {
		return nil, err
	}

	// Check version number against old manifest
	var oldIndex v1manifest.Index
	exists, err := r.local.LoadManifest(&oldIndex)
	if exists {
		if err != nil {
			return nil, err
		}
		if index.Version <= oldIndex.Version {
			return nil, fmt.Errorf("index manifest has a version number <= the old manifest (%v, %v)", index.Version, oldIndex.Version)
		}
	}

	err = r.local.SaveManifest(manifest, v1manifest.ManifestFilenameIndex)
	if err != nil {
		return nil, err
	}

	return &index, nil
}

// Precondition: the snapshot manifest exists and is up to date
func (r *V1Repository) updateComponentManifest(id string) (*v1manifest.Component, error) {
	// Find the component's entry in the index and snapshot manifests.
	var index v1manifest.Index
	_, err := r.local.LoadManifest(&index)
	if err != nil {
		return nil, err
	}
	item := index.Components[id]
	var snapshot v1manifest.Snapshot
	_, err = r.local.LoadManifest(&snapshot)
	if err != nil {
		return nil, err
	}

	url, err := snapshot.VersionedURL(item.URL)
	if err != nil {
		return nil, err
	}
	var component v1manifest.Component
	manifest, err := r.fetchManifest(url, &component, r.local.Keys(), snapshot.Meta[item.URL].Length)
	if err != nil {
		return nil, err
	}

	filename := v1manifest.ComponentFilename(id)
	oldManifest, err := r.local.LoadComponentManifest(filename)
	if err != nil {
		return nil, err
	}
	if oldManifest != nil {
		if component.Version <= oldManifest.Version {
			return nil, fmt.Errorf("component manifest for %s has a version number <= the old manifest (%v, %v)", id, component.Version, oldManifest.Version)
		}
	}

	err = r.local.SaveComponentManifest(manifest, filename)
	if err != nil {
		return nil, err
	}

	return &component, nil
}

// downloadComponent downloads a component using the relevant manifest and checks its hash.
// Precondition: the component's manifest is up to date and the version and platform are valid.
func (r *V1Repository) downloadComponent(id string, platform string, version string) (io.Reader, error) {
	manifest, err := r.local.LoadComponentManifest(v1manifest.ComponentFilename(id))
	if err != nil {
		return nil, err
	}

	item := manifest.Platforms[platform][version]

	reader, err := r.mirror.Fetch(item.URL, int64(item.Length))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	buffer := new(bytes.Buffer)
	_, err = io.Copy(buffer, reader)
	if err != nil {
		return nil, err
	}

	bufReader := bytes.NewReader(buffer.Bytes())
	if err = utils.CheckSHA256(bufReader, item.Hashes[v1manifest.SHA256]); err != nil {
		fmt.Println(err)
		return nil, err
	}

	_, err = bufReader.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}

	return bufReader, nil
}

// CheckTimestamp downloads the timestamp file, validates it, and checks if the snapshot hash matches our local one.
// If they match, then there is nothing to update and we return nil. If they do not match, we return the
// snapshot's file info.
func (r *V1Repository) checkTimestamp() (*v1manifest.FileHash, error) {
	var ts v1manifest.Timestamp
	manifest, err := r.fetchManifest(v1manifest.ManifestURLTimestamp, &ts, r.local.Keys(), maxTimeStampSize)
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

	return &hash, r.local.SaveManifest(manifest, v1manifest.ManifestFilenameTimestamp)
}

// fetchManifest downloads and validates a manifest from this repo.
func (r *V1Repository) fetchManifest(url string, role v1manifest.ValidManifest, keys crypto.KeyStore, maxSize uint) (*v1manifest.Manifest, error) {
	reader, err := r.mirror.Fetch(url, int64(maxSize))
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer reader.Close()
	return v1manifest.ReadManifest(reader, role, keys)
}
