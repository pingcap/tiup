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
	"github.com/pingcap-incubator/tiup/pkg/repository/crypto"
	"github.com/pingcap-incubator/tiup/pkg/repository/manifest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckTimestamp(t *testing.T) {
	mirror := MockMirror{
		Resources: map[string]string{},
	}
	repo := NewV1Repo(&mirror, Options{})
	local := manifest.MockManifests{
		Manifests: map[string]manifest.ValidManifest{},
	}

	repoTimestamp := timestampManifest()
	// Test that no local timestamp => return hash
	mirror.Resources["timestamp.json"] = serialize(t, repoTimestamp)
	hash, err := repo.checkTimestamp(&local)
	assert.Nil(t, err)
	assert.Equal(t, uint(1001), hash.Length)
	assert.Equal(t, "123456", hash.Hashes["sha256"])

	// Test that hashes match => return nil
	localManifest := timestampManifest()
	localManifest.Version = 43
	localManifest.Expires = "2220-05-13T04:51:08Z"
	local.Manifests["timestamp.json"] = localManifest
	hash, err = repo.checkTimestamp(&local)
	assert.Nil(t, err)
	assert.Nil(t, hash)

	// Hashes don't match => return correct File hash
	localManifest.Meta["snapshot.json"].Hashes["sha256"] = "023456"
	hash, err = repo.checkTimestamp(&local)
	assert.Nil(t, err)
	assert.Equal(t, uint(1001), hash.Length)
	assert.Equal(t, "123456", hash.Hashes["sha256"])

	// Test that an expired manifest from the mirror causes an error
	expiredTimestamp := timestampManifest()
	expiredTimestamp.Expires = "2000-05-12T04:51:08Z"
	mirror.Resources["timestamp.json"] = serialize(t, expiredTimestamp)
	hash, err = repo.checkTimestamp(&local)
	assert.NotNil(t, err)

	// Test that an invalid manifest from the mirror causes an error
	invalidTimestamp := timestampManifest()
	invalidTimestamp.SpecVersion = "10.1.0"
	hash, err = repo.checkTimestamp(&local)
	assert.NotNil(t, err)

	// TODO test that a bad signature causes an error
}

func TestUpdateLocalSnapshot(t *testing.T) {
	mirror := MockMirror{
		Resources: map[string]string{},
	}
	repo := NewV1Repo(&mirror, Options{})
	local := manifest.MockManifests{
		Manifests: map[string]manifest.ValidManifest{},
	}

	timestamp := timestampManifest()
	snapshotManifest := snapshotManifest()
	mirror.Resources["timestamp.json"] = serialize(t, timestamp)
	mirror.Resources["snapshot.json"] = serialize(t, snapshotManifest)
	local.Manifests["timestamp.json"] = timestamp

	// test that up to date timestamp does nothing
	snapshot, err := repo.updateLocalSnapshot(&local)
	assert.Nil(t, err)
	assert.Nil(t, snapshot)

	// test that out of date timestamp downloads and saves snapshot
	timestamp.Meta["snapshot.json"].Hashes["sha256"] = "an old hash"
	timestamp.Version -= 1
	snapshot, err = repo.updateLocalSnapshot(&local)
	assert.Nil(t, err)
	assert.NotNil(t, snapshot)

	// test that invalid snapshot causes an error
	snapshotManifest.Expires = "2000-05-11T04:51:08Z"
	mirror.Resources["snapshot.json"] = serialize(t, snapshotManifest)
	snapshot, err = repo.updateLocalSnapshot(&local)
	assert.NotNil(t, err)
	assert.Nil(t, snapshot)

	// TODO test that invalid signature of snapshot causes an error
	// TODO test that signature error on timestamp causes root to be reloaded and timestamp to be rechecked
}

func timestampManifest() *manifest.Timestamp {
	return &manifest.Timestamp{
		SignedBase: manifest.SignedBase{
			Ty:          "timestamp",
			SpecVersion: "0.1.0",
			Expires:     "2220-05-11T04:51:08Z",
			Version:     42,
		},
		Meta: map[string]manifest.FileHash{"snapshot.json": {
			Hashes: map[string]string{"sha256": "123456"},
			Length: 1001,
		}},
	}
}

func snapshotManifest() *manifest.Snapshot {
	return &manifest.Snapshot{
		SignedBase: manifest.SignedBase{
			Ty:          "snapshot",
			SpecVersion: "0.1.0",
			Expires:     "2220-05-11T04:51:08Z",
			Version:     42,
		},
		Meta: map[string]manifest.FileVersion{"root": {Version: 56}},
	}
}

func serialize(t *testing.T, role manifest.ValidManifest) string {
	var out strings.Builder
	_, priv, err := crypto.RsaPair()
	assert.Nil(t, err)
	err = manifest.SignAndWrite(&out, role, "KEYID", priv)
	assert.Nil(t, err)
	return out.String()
}
