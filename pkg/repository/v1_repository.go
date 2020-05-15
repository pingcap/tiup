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
	"runtime"

	"github.com/pingcap-incubator/tiup/pkg/repository/crypto"
	"github.com/pingcap-incubator/tiup/pkg/repository/v1manifest"
	"github.com/pingcap/errors"
)

// V1Repository represents a remote repository viewed with the v1 manifest design.
type V1Repository struct {
	Options
	mirror Mirror
}

// NewV1Repo creates a new v1 repository from the given mirror
func NewV1Repo(mirror Mirror, opts Options) *V1Repository {
	if opts.GOOS == "" {
		opts.GOOS = runtime.GOOS
	}
	if opts.GOARCH == "" {
		opts.GOARCH = runtime.GOARCH
	}

	return &V1Repository{
		Options: opts,
		mirror:  mirror,
	}
}

const maxTimeStampSize uint = 1024

// If the snapshot has been updated, we return the new snapshot, if not we return nil.
// Postcondition: if returned error is nil, then the local snapshot and timestamp are up to date.
func (r *V1Repository) updateLocalSnapshot(local v1manifest.LocalManifests) (*v1manifest.Snapshot, error) {
	hash, err := r.checkTimestamp(local)
	if _, ok := err.(*v1manifest.SignatureError); ok {
		// The signature is wrong, update our signatures from the root manifest and try again.
		err = r.updateLocalRoot(local)
		if err != nil {
			return nil, err
		}
		hash, err = r.checkTimestamp(local)
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
	manifest, err := r.FetchManifest(v1manifest.ManifestURLSnapshot, &snapshot, local.Keys(), hash.Length)
	if err != nil {
		return nil, err
	}

	// TODO validate snapshot against hash

	err = local.SaveManifest(manifest, v1manifest.ManifestFilenameSnapshot)
	if err != nil {
		return nil, err
	}

	return &snapshot, nil
}

func (r *V1Repository) updateLocalRoot(local v1manifest.LocalManifests) error {
	// When we save to disk need to save twice:
	//filename := v1manifest.ManifestFilenameRoot
	//err := local.SaveManifest(manifest, filename)
	//version := manifest.Signed.Base().Version
	//filename =  fmt.Sprintf("%v.%s", version, filename)
	//err = local.SaveManifest(manifest, filename)
	return nil
}

// Precondition: the index manifest actually requires updating, the root manifest has been updated if necessary.
func (r *V1Repository) updateLocalIndex(local v1manifest.LocalManifests, length uint) (*v1manifest.Index, error) {
	var root v1manifest.Root
	_, err := local.LoadManifest(&root)
	if err != nil {
		return nil, err
	}

	var snapshot v1manifest.Snapshot
	_, err = local.LoadManifest(&snapshot)
	if err != nil {
		return nil, err
	}

	url, err := snapshot.VersionedURL(root.Roles[v1manifest.ManifestTypeIndex].URL)
	if err != nil {
		return nil, err
	}

	var index v1manifest.Index
	manifest, err := r.FetchManifest(url, &index, local.Keys(), length)
	if err != nil {
		return nil, err
	}

	// Check version number against old manifest
	var oldIndex v1manifest.Index
	exists, err := local.LoadManifest(&oldIndex)
	if exists {
		if err != nil {
			return nil, err
		}
		if index.Version <= oldIndex.Version {
			fmt.Errorf("index manifest has a version number <= the old manifest (%v, %v)", index.Version, oldIndex.Version)
		}
	}

	err = local.SaveManifest(manifest, v1manifest.ManifestFilenameIndex)
	if err != nil {
		return nil, err
	}

	return &index, nil
}

// Precondition: the snapshot manifest exists and is up to date
func (r *V1Repository) updateComponentManifest(local v1manifest.LocalManifests, id string) (*v1manifest.Component, error) {
	// Find the component's entry in the index and snapshot manifests.
	var index v1manifest.Index
	_, err := local.LoadManifest(&index)
	if err != nil {
		return nil, err
	}
	item := index.Components[id]
	var snapshot v1manifest.Snapshot
	_, err = local.LoadManifest(&snapshot)
	if err != nil {
		return nil, err
	}

	url, err := snapshot.VersionedURL(item.URL)
	if err != nil {
		return nil, err
	}
	var component v1manifest.Component
	// TODO length should be in snapshot
	manifest, err := r.FetchManifest(url, &component, local.Keys(), 0)
	if err != nil {
		return nil, err
	}

	filename := v1manifest.ComponentFilename(id)
	oldManifest, err := local.LoadComponentManifest(filename)
	if err != nil {
		return nil, err
	}
	if oldManifest != nil {
		if component.Version <= oldManifest.Version {
			return nil, fmt.Errorf("component manifest for %s has a version number <= the old manifest (%v, %v)", id, component.Version, oldManifest.Version)
		}
	}

	err = local.SaveComponentManifest(manifest, filename)
	if err != nil {
		return nil, err
	}

	return &component, nil
}

// CheckTimestamp downloads the timestamp file, validates it, and checks if the snapshot hash matches our local one.
// If they match, then there is nothing to update and we return nil. If they do not match, we return the
// snapshot's file info.
func (r *V1Repository) checkTimestamp(local v1manifest.LocalManifests) (*v1manifest.FileHash, error) {
	var ts v1manifest.Timestamp
	manifest, err := r.FetchManifest(v1manifest.ManifestURLTimestamp, &ts, local.Keys(), maxTimeStampSize)
	if err != nil {
		return nil, err
	}
	hash := ts.SnapshotHash()

	var localTs v1manifest.Timestamp
	exists, err := local.LoadManifest(&localTs)
	if !exists {
		// We can't find a local timestamp, so we're going to have to update
		return &hash, local.SaveManifest(manifest, v1manifest.ManifestFilenameTimestamp)
	} else if err != nil {
		return nil, err
	}
	if hash.Hashes["sha256"] == localTs.SnapshotHash().Hashes["sha256"] {
		return nil, nil
	}

	return &hash, local.SaveManifest(manifest, v1manifest.ManifestFilenameTimestamp)
}

// FetchManifest downloads and validates a manifest from this repo.
func (r *V1Repository) FetchManifest(url string, role v1manifest.ValidManifest, keys crypto.KeyStore, maxSize uint) (*v1manifest.Manifest, error) {
	reader, err := r.mirror.Fetch(url, int64(maxSize))
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer reader.Close()
	return v1manifest.ReadManifest(reader, role, keys)
}
