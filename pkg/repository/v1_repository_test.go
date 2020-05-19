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
	"io"
	"strings"
	"testing"

	"github.com/pingcap-incubator/tiup/pkg/repository/crypto"
	"github.com/pingcap-incubator/tiup/pkg/repository/v1manifest"
	"github.com/stretchr/testify/assert"
)

func TestFnameWithVersion(t *testing.T) {
	tests := []struct {
		name        string
		version     uint
		versionName string
	}{
		{"root.json", 1, "1.root.json"},
		{"/root.json", 1, "/1.root.json"},
	}

	for _, test := range tests {
		fname := fnameWithVersion(test.name, test.version)
		assert.Equal(t, test.versionName, fname)
	}
}

func TestCheckTimestamp(t *testing.T) {
	mirror := MockMirror{
		Resources: map[string]string{},
	}
	local := v1manifest.NewMockManifests()
	repo := NewV1Repo(&mirror, Options{}, local)

	repoTimestamp := timestampManifest()
	// Test that no local timestamp => return hash
	mirror.Resources[v1manifest.ManifestURLTimestamp] = serialize(t, repoTimestamp)
	hash, err := repo.checkTimestamp()
	assert.Nil(t, err)
	assert.Equal(t, uint(1001), hash.Length)
	assert.Equal(t, "123456", hash.Hashes[v1manifest.SHA256])
	assert.Contains(t, local.Saved, v1manifest.ManifestFilenameTimestamp)

	// Test that hashes match => return nil
	localManifest := timestampManifest()
	localManifest.Version = 43
	localManifest.Expires = "2220-05-13T04:51:08Z"
	local.Manifests[v1manifest.ManifestFilenameTimestamp] = localManifest
	local.Saved = []string{}
	hash, err = repo.checkTimestamp()
	assert.Nil(t, err)
	assert.Nil(t, hash)
	assert.Empty(t, local.Saved)

	// Hashes don't match => return correct File hash
	localManifest.Meta[v1manifest.ManifestURLSnapshot].Hashes[v1manifest.SHA256] = "023456"
	hash, err = repo.checkTimestamp()
	assert.Nil(t, err)
	assert.Equal(t, uint(1001), hash.Length)
	assert.Equal(t, "123456", hash.Hashes[v1manifest.SHA256])
	assert.Contains(t, local.Saved, v1manifest.ManifestFilenameTimestamp)

	// Test that an expired manifest from the mirror causes an error
	expiredTimestamp := timestampManifest()
	expiredTimestamp.Expires = "2000-05-12T04:51:08Z"
	mirror.Resources[v1manifest.ManifestURLTimestamp] = serialize(t, expiredTimestamp)
	local.Saved = []string{}
	hash, err = repo.checkTimestamp()
	assert.NotNil(t, err)
	assert.Empty(t, local.Saved)

	// Test that an invalid manifest from the mirror causes an error
	invalidTimestamp := timestampManifest()
	invalidTimestamp.SpecVersion = "10.1.0"
	hash, err = repo.checkTimestamp()
	assert.NotNil(t, err)
	assert.Empty(t, local.Saved)

	// TODO test that a bad signature causes an error
}

func TestUpdateLocalSnapshot(t *testing.T) {
	mirror := MockMirror{
		Resources: map[string]string{},
	}
	local := v1manifest.NewMockManifests()
	repo := NewV1Repo(&mirror, Options{}, local)

	timestamp := timestampManifest()
	snapshotManifest := snapshotManifest()
	mirror.Resources[v1manifest.ManifestURLTimestamp] = serialize(t, timestamp)
	mirror.Resources[v1manifest.ManifestURLSnapshot] = serialize(t, snapshotManifest)
	local.Manifests[v1manifest.ManifestFilenameTimestamp] = timestamp

	// test that up to date timestamp does nothing
	snapshot, err := repo.updateLocalSnapshot()
	assert.Nil(t, err)
	assert.Nil(t, snapshot)
	assert.Empty(t, local.Saved)

	// test that out of date timestamp downloads and saves snapshot
	timestamp.Meta[v1manifest.ManifestURLSnapshot].Hashes[v1manifest.SHA256] = "an old hash"
	timestamp.Version -= 1
	snapshot, err = repo.updateLocalSnapshot()
	assert.Nil(t, err)
	assert.NotNil(t, snapshot)
	assert.Contains(t, local.Saved, v1manifest.ManifestFilenameSnapshot)

	// test that invalid snapshot causes an error
	snapshotManifest.Expires = "2000-05-11T04:51:08Z"
	mirror.Resources[v1manifest.ManifestURLSnapshot] = serialize(t, snapshotManifest)
	local.Saved = []string{}
	snapshot, err = repo.updateLocalSnapshot()
	assert.NotNil(t, err)
	assert.Nil(t, snapshot)
	assert.NotContains(t, local.Saved, v1manifest.ManifestFilenameSnapshot)

	// TODO test that invalid signature of snapshot causes an error
	// TODO test that signature error on timestamp causes root to be reloaded and timestamp to be rechecked
}

func TestUpdateLocalRoot(t *testing.T) {
	mirror := MockMirror{
		Resources: map[string]string{},
	}

	local := v1manifest.NewMockManifests()
	repo := NewV1Repo(&mirror, Options{}, local)

	root, privKey := rootManifest(t)
	local.Manifests[v1manifest.ManifestFilenameRoot] = root

	// Should success if no new version root.
	err := repo.updateLocalRoot()
	assert.Nil(t, err)

	root2, privKey2 := rootManifest(t)
	fname := fmt.Sprintf("/%d.root.json", root.Version+1)
	mirror.Resources[fname] = serialize(t, root2, privKey, privKey2)

	// Fail cause wrong version
	err = repo.updateLocalRoot()
	assert.NotNil(t, err)

	// Fix Version but the new root expired.
	root2.Version = root.Version + 1
	root2.Expires = "2000-05-11T04:51:08Z"
	mirror.Resources[fname] = serialize(t, root2, privKey, privKey2)
	err = repo.updateLocalRoot()
	assert.NotNil(t, err)

	// Fix Expires, should success now.
	root2.Expires = "2222-05-11T04:51:08Z"
	mirror.Resources[fname] = serialize(t, root2, privKey, privKey2)
	err = repo.updateLocalRoot()
	assert.Nil(t, err)
}

func TestUpdateIndex(t *testing.T) {
	// Test that updating succeeds with a valid manifest and local manifests.
	mirror := MockMirror{
		Resources: map[string]string{},
	}
	local := v1manifest.NewMockManifests()
	repo := NewV1Repo(&mirror, Options{}, local)

	index := indexManifest()
	root, _ := rootManifest(t)
	snapshot := snapshotManifest()
	serIndex := serialize(t, index)
	mirror.Resources["/5.index.json"] = serIndex
	local.Manifests[v1manifest.ManifestFilenameRoot] = root
	local.Manifests[v1manifest.ManifestFilenameSnapshot] = snapshot
	index.Version -= 1
	local.Manifests[v1manifest.ManifestFilenameIndex] = index

	updated, err := repo.updateLocalIndex(uint(len(serIndex)))
	assert.Nil(t, err)
	assert.NotNil(t, updated)
	assert.Contains(t, local.Saved, "index.json")

	// TODO test that invalid signature of snapshot causes an error
}

func TestUpdateComponent(t *testing.T) {
	mirror := MockMirror{
		Resources: map[string]string{},
	}
	local := v1manifest.NewMockManifests()
	repo := NewV1Repo(&mirror, Options{}, local)

	index := indexManifest()
	root, _ := rootManifest(t)
	snapshot := snapshotManifest()
	foo := componentManifest()
	serFoo := serialize(t, foo)
	mirror.Resources["/7.foo.json"] = serFoo
	local.Manifests[v1manifest.ManifestFilenameRoot] = root
	local.Manifests[v1manifest.ManifestFilenameSnapshot] = snapshot
	local.Manifests[v1manifest.ManifestFilenameIndex] = index

	// Test happy path
	updated, err := repo.updateComponentManifest("foo")
	assert.Nil(t, err)
	assert.NotNil(t, updated)
	assert.Contains(t, local.Saved, "foo.json")

	// Test that decrementing version numbers cause an error
	oldFoo := componentManifest()
	oldFoo.Version = 8
	local.Manifests["foo.json"] = oldFoo
	local.Saved = []string{}
	updated, err = repo.updateComponentManifest("foo")
	assert.NotNil(t, err)
	assert.Nil(t, updated)
	assert.Empty(t, local.Saved)

	// Test that id missing from index causes an error
	updated, err = repo.updateComponentManifest("bar")
	assert.NotNil(t, err)
	assert.Nil(t, updated)
	assert.Empty(t, local.Saved)

	// TODO check that the correct files were created
	// TODO test that invalid signature of component manifest causes an error
}

func TestDownloadManifest(t *testing.T) {
	mirror := MockMirror{
		Resources: map[string]string{},
	}
	someString := "just some string for testing"
	mirror.Resources["/foo-2.0.1.tar.gz"] = someString
	local := v1manifest.NewMockManifests()
	foo := componentManifest()
	local.Manifests["foo.json"] = foo
	repo := NewV1Repo(&mirror, Options{}, local)

	// Happy path file is as expected
	reader, err := repo.downloadComponent("foo", "a_platform", "2.0.1")
	assert.Nil(t, err)
	buf := new(strings.Builder)
	_, err = io.Copy(buf, reader)
	assert.Nil(t, err)
	assert.Equal(t, someString, buf.String())

	// Sad paths
	version1 := foo.Platforms["a_platform"]["2.0.1"]
	version2 := foo.Platforms["a_platform"]["2.0.1"]
	version3 := foo.Platforms["a_platform"]["2.0.1"]

	// bad hash
	version1.Hashes[v1manifest.SHA256] = "Not a hash"
	foo.Platforms["a_platform"]["2.0.1"] = version1
	_, err = repo.downloadComponent("foo", "a_platform", "2.0.1")
	assert.NotNil(t, err)

	//  Too long
	version2.Length = 26
	foo.Platforms["a_platform"]["2.0.1"] = version2
	_, err = repo.downloadComponent("foo", "a_platform", "2.0.1")
	assert.NotNil(t, err)

	// missing tar ball/bad url
	version3.URL = "/bar-2.0.1.tar.gz"
	foo.Platforms["a_platform"]["2.0.1"] = version3
	_, err = repo.downloadComponent("foo", "a_platform", "2.0.1")
	assert.NotNil(t, err)
}

func timestampManifest() *v1manifest.Timestamp {
	return &v1manifest.Timestamp{
		SignedBase: v1manifest.SignedBase{
			Ty:          v1manifest.ManifestTypeTimestamp,
			SpecVersion: "0.1.0",
			Expires:     "2220-05-11T04:51:08Z",
			Version:     42,
		},
		Meta: map[string]v1manifest.FileHash{v1manifest.ManifestURLSnapshot: {
			Hashes: map[string]string{v1manifest.SHA256: "123456"},
			Length: 1001,
		}},
	}
}

func snapshotManifest() *v1manifest.Snapshot {
	return &v1manifest.Snapshot{
		SignedBase: v1manifest.SignedBase{
			Ty:          v1manifest.ManifestTypeSnapshot,
			SpecVersion: "0.1.0",
			Expires:     "2220-05-11T04:51:08Z",
			Version:     42,
		},
		Meta: map[string]v1manifest.FileVersion{
			v1manifest.ManifestURLRoot:  {Version: 56},
			v1manifest.ManifestURLIndex: {Version: 5},
			"/foo.json":                 {Version: 7},
		},
	}
}

func componentManifest() *v1manifest.Component {
	return &v1manifest.Component{
		SignedBase: v1manifest.SignedBase{
			Ty:          v1manifest.ManifestTypeComponent,
			SpecVersion: "0.1.0",
			Expires:     "2220-05-11T04:51:08Z",
			Version:     7,
		},
		Name:        "Foo",
		Description: "foo does stuff",
		Platforms: map[string]map[string]v1manifest.VersionItem{
			"a_platform": {"2.0.1": {
				URL: "/foo-2.0.1.tar.gz",
				FileHash: v1manifest.FileHash{
					Hashes: map[string]string{v1manifest.SHA256: "963ba8374bac92a8a00fc21ca458e0c2016bf8930519e5271f7b49d16762a184"},
					Length: 28,
				},
			}},
		},
	}
}

func indexManifest() *v1manifest.Index {
	return &v1manifest.Index{
		SignedBase: v1manifest.SignedBase{
			Ty:          v1manifest.ManifestTypeIndex,
			SpecVersion: "0.1.0",
			Expires:     "2220-05-11T04:51:08Z",
			Version:     5,
		},
		Owners: map[string]v1manifest.Owner{"bar": {
			Name: "Bar",
			Keys: nil,
		}},
		Components: map[string]v1manifest.ComponentItem{"foo": {
			Yanked:    false,
			Owner:     "bar",
			URL:       "/foo.json",
			Threshold: 1,
		}},
		DefaultComponents: []string{},
	}
}

func rootManifest(t *testing.T) (*v1manifest.Root, crypto.PrivKey) {
	// TODO use the key id and private key to sign the index manifest
	info, keyID, priv, err := v1manifest.FreshKeyInfo()
	assert.Nil(t, err)
	id, err := info.ID()
	assert.Nil(t, err)
	bytes, err := priv.Serialize()
	assert.Nil(t, err)
	privKeyInfo := v1manifest.NewKeyInfo(bytes)
	// The signed id will be priveID and it should be equal as keyID
	privID, err := privKeyInfo.ID()
	assert.Nil(t, err)
	assert.Equal(t, keyID, privID)

	t.Log("keyID: ", keyID)
	t.Log("id: ", id)
	t.Log("privKeyInfo id: ", privID)
	// t.Logf("info: %+v\n", info)
	// t.Logf("pinfo: %+v\n", privKeyInfo)

	return &v1manifest.Root{
		SignedBase: v1manifest.SignedBase{
			Ty:          v1manifest.ManifestTypeRoot,
			SpecVersion: "0.1.0",
			Expires:     "2220-05-11T04:51:08Z",
			Version:     42,
		},
		Roles: map[string]*v1manifest.Role{
			v1manifest.ManifestTypeIndex: {
				URL:       v1manifest.ManifestURLIndex,
				Keys:      map[string]*v1manifest.KeyInfo{keyID: info},
				Threshold: 1,
			},
			v1manifest.ManifestTypeRoot: {
				URL:       v1manifest.ManifestURLRoot,
				Keys:      map[string]*v1manifest.KeyInfo{keyID: info},
				Threshold: 1,
			},
		},
	}, priv
}

func serialize(t *testing.T, role v1manifest.ValidManifest, privKeys ...crypto.PrivKey) string {
	var keyInfos []*v1manifest.KeyInfo

	var priv crypto.PrivKey
	if len(privKeys) > 0 {
		for _, priv := range privKeys {
			bytes, err := priv.Serialize()
			assert.Nil(t, err)
			keyInfo := v1manifest.NewKeyInfo(bytes)
			keyInfos = append(keyInfos, keyInfo)
		}
	} else {
		// just use a generate one
		var err error
		_, priv, err = crypto.RSAPair()
		assert.Nil(t, err)
		bytes, err := priv.Serialize()
		assert.Nil(t, err)
		keyInfo := v1manifest.NewKeyInfo(bytes)
		keyInfos = append(keyInfos, keyInfo)
	}

	var out strings.Builder
	err := v1manifest.SignAndWrite(&out, role, keyInfos...)
	assert.Nil(t, err)
	return out.String()
}
