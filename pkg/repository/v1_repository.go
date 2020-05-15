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
	"github.com/pingcap-incubator/tiup/pkg/repository/manifest"
	"runtime"

	"github.com/pingcap-incubator/tiup/pkg/repository/crypto"
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
func (r *V1Repository) updateLocalSnapshot(local manifest.LocalManifests) (*manifest.Snapshot, error) {
	hash, err := r.checkTimestamp(local)
	if _, ok := err.(*manifest.SignatureError); ok {
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

	var snapshot manifest.Snapshot
	manifest, err := r.FetchManifest(snapshot.Filename(), &snapshot, local.Keys(), hash.Length)
	if err != nil {
		return nil, err
	}

	// TODO validate snapshot against hash

	err = local.SaveManifest(manifest)
	if err != nil {
		return nil, err
	}

	return &snapshot, nil
}

func (r *V1Repository) updateLocalRoot(local manifest.LocalManifests) error {
	return nil
}

func (r *V1Repository) updateLocalIndex(local manifest.LocalManifests) error {
	return nil
}

// CheckTimestamp downloads the timestamp file, validates it, and checks if the snapshot hash matches our local one.
// If they match, then there is nothing to update and we return nil. If they do not match, we return the
// snapshot's file info.
func (r *V1Repository) checkTimestamp(local manifest.LocalManifests) (*manifest.FileHash, error) {
	var ts manifest.Timestamp
	_, err := r.FetchManifest(ts.Filename(), &ts, local.Keys(), maxTimeStampSize)
	if err != nil {
		return nil, err
	}
	hash := ts.SnapshotHash()

	var localTs manifest.Timestamp
	err = local.LoadManifest(&localTs)
	if err != nil {
		// We can't find a local timestamp, so we're going to have to update
		return &hash, nil
	}
	if hash.Hashes["sha256"] == localTs.SnapshotHash().Hashes["sha256"] {
		return nil, nil
	}

	return &hash, nil
}

// FetchManifest downloads and validates a manifest from this repo.
func (r *V1Repository) FetchManifest(filename string, role manifest.ValidManifest, keys *crypto.KeyStore, maxSize uint) (*manifest.Manifest, error) {
	reader, err := r.mirror.Fetch(filename, int64(maxSize))
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer reader.Close()
	return manifest.ReadManifest(reader, role, keys)
}
