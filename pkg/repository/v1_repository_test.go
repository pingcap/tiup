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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"testing"

	cjson "github.com/gibson042/canonicaljson-go"
	"github.com/pingcap/tiup/pkg/repository/crypto"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
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
		fname := FnameWithVersion(test.name, test.version)
		assert.Equal(t, test.versionName, fname)
	}
}

func TestCheckTimestamp(t *testing.T) {
	mirror := MockMirror{
		Resources: map[string]string{},
	}
	local := v1manifest.NewMockManifests()
	privk := setNewRoot(t, local)
	repo := NewV1Repo(&mirror, Options{}, local)

	repoTimestamp := timestampManifest()
	// Test that no local timestamp => return changed = true
	mirror.Resources[v1manifest.ManifestURLTimestamp] = serialize(t, repoTimestamp, privk)
	changed, manifest, err := repo.fetchTimestamp()
	assert.Nil(t, err)
	err = local.SaveManifest(manifest, v1manifest.ManifestFilenameTimestamp)
	assert.Nil(t, err)
	assert.NotNil(t, manifest)
	tmp := manifest.Signed.(*v1manifest.Timestamp).SnapshotHash()
	hash := &tmp
	assert.NotNil(t, hash)
	assert.Equal(t, changed, true)
	assert.Equal(t, uint(1001), hash.Length)
	assert.Equal(t, "123456", hash.Hashes[v1manifest.SHA256])
	assert.Contains(t, local.Saved, v1manifest.ManifestFilenameTimestamp)

	changed, manifest, err = repo.fetchTimestamp()
	assert.Nil(t, err)
	err = local.SaveManifest(manifest, v1manifest.ManifestFilenameTimestamp)
	assert.Nil(t, err)
	assert.NotNil(t, manifest)
	tmp = manifest.Signed.(*v1manifest.Timestamp).SnapshotHash()
	hash = &tmp
	assert.NotNil(t, hash)
	assert.Equal(t, changed, false)

	// Test that an expired manifest from the mirror causes an error
	expiredTimestamp := timestampManifest()
	expiredTimestamp.Expires = "2000-05-12T04:51:08Z"
	mirror.Resources[v1manifest.ManifestURLTimestamp] = serialize(t, expiredTimestamp)
	_, _, err = repo.fetchTimestamp()
	assert.NotNil(t, err)

	// Test that an invalid manifest from the mirror causes an error
	invalidTimestamp := timestampManifest()
	invalidTimestamp.SpecVersion = "10.1.0"
	_, _, err = repo.fetchTimestamp()
	assert.NotNil(t, err)

	// TODO test that a bad signature causes an error
}

func TestUpdateLocalSnapshot(t *testing.T) {
	mirror := MockMirror{
		Resources: map[string]string{},
	}
	local := v1manifest.NewMockManifests()
	privk := setNewRoot(t, local)
	repo := NewV1Repo(&mirror, Options{}, local)

	timestamp := timestampManifest()
	snapshotManifest := snapshotManifest()
	serSnap := serialize(t, snapshotManifest, privk)
	mirror.Resources[v1manifest.ManifestURLSnapshot] = serSnap
	timestamp.Meta[v1manifest.ManifestURLSnapshot].Hashes[v1manifest.SHA256] = hash(serSnap)

	signedTs := serialize(t, timestamp, privk)
	mirror.Resources[v1manifest.ManifestURLTimestamp] = signedTs
	ts := &v1manifest.Manifest{Signed: &v1manifest.Timestamp{}}
	err := cjson.Unmarshal([]byte(signedTs), ts)
	assert.Nil(t, err)
	local.Manifests[v1manifest.ManifestFilenameTimestamp] = ts

	// test that out of date timestamp downloads and saves snapshot
	timestamp.Meta[v1manifest.ManifestURLSnapshot].Hashes[v1manifest.SHA256] = "an old hash"
	timestamp.Version--
	snapshot, err := repo.updateLocalSnapshot()
	assert.Nil(t, err)
	assert.NotNil(t, snapshot)
	assert.Contains(t, local.Saved, v1manifest.ManifestFilenameSnapshot)

	// test that if snapshot unchanged, no update of snapshot is performed
	timestamp.Meta[v1manifest.ManifestURLSnapshot].Hashes[v1manifest.SHA256] = hash(serSnap)
	timestamp.Version += 2
	mirror.Resources[v1manifest.ManifestURLTimestamp] = serialize(t, timestamp, privk)

	local.Saved = local.Saved[:0]
	snapshot, err = repo.updateLocalSnapshot()
	assert.Nil(t, err)
	assert.NotNil(t, snapshot)
	assert.NotContains(t, local.Saved, v1manifest.ManifestFilenameSnapshot)

	// delete the local snapshot will fetch and save it again
	delete(local.Manifests, v1manifest.ManifestFilenameSnapshot)
	snapshot, err = repo.updateLocalSnapshot()
	assert.Nil(t, err)
	assert.NotNil(t, snapshot)
	assert.Contains(t, local.Saved, v1manifest.ManifestFilenameSnapshot)

	// test that invalid snapshot will causes an error
	snapshotManifest.Expires = "2000-05-11T04:51:08Z"
	mirror.Resources[v1manifest.ManifestURLSnapshot] = serialize(t, snapshotManifest, privk)
	local.Manifests[v1manifest.ManifestFilenameTimestamp] = ts
	delete(local.Manifests, v1manifest.ManifestFilenameSnapshot)
	local.Saved = local.Saved[:0]
	snapshot, err = repo.updateLocalSnapshot()
	assert.NotNil(t, err)
	assert.Nil(t, snapshot)
	assert.NotContains(t, local.Saved, v1manifest.ManifestFilenameSnapshot)

	// TODO test an expired manifest will cause an error or force update
	// TODO test that invalid signature of snapshot causes an error
	// TODO test that signature error on timestamp causes root to be reloaded and timestamp to be rechecked
}

func TestUpdateLocalRoot(t *testing.T) {
	mirror := MockMirror{
		Resources: map[string]string{},
	}

	local := v1manifest.NewMockManifests()
	privKey := setNewRoot(t, local)
	repo := NewV1Repo(&mirror, Options{}, local)

	// Should success if no new version root.
	err := repo.updateLocalRoot()
	assert.Nil(t, err)

	root2, privKey2 := rootManifest(t)
	root, _ := repo.loadRoot()
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
	priv := setNewRoot(t, local)
	repo := NewV1Repo(&mirror, Options{}, local)

	index, _ := indexManifest(t)
	snapshot := snapshotManifest()
	serIndex := serialize(t, index, priv)
	mirror.Resources["/5.index.json"] = serIndex
	local.Manifests[v1manifest.ManifestFilenameSnapshot] = &v1manifest.Manifest{Signed: snapshot}
	index.Version--
	local.Manifests[v1manifest.ManifestFilenameIndex] = &v1manifest.Manifest{Signed: index}

	err := repo.updateLocalIndex(snapshot)
	assert.Nil(t, err)
	assert.Contains(t, local.Saved, "index.json")

	// TODO test that invalid signature of snapshot causes an error
}

func TestUpdateComponent(t *testing.T) {
	mirror := MockMirror{
		Resources: map[string]string{},
	}
	local := v1manifest.NewMockManifests()
	_ = setNewRoot(t, local)
	repo := NewV1Repo(&mirror, Options{}, local)

	index, indexPriv := indexManifest(t)
	snapshot := snapshotManifest()
	foo := componentManifest()
	serFoo := serialize(t, foo, indexPriv)
	mirror.Resources["/7.foo.json"] = serFoo
	local.Manifests[v1manifest.ManifestFilenameSnapshot] = &v1manifest.Manifest{Signed: snapshot}
	local.Manifests[v1manifest.ManifestFilenameIndex] = &v1manifest.Manifest{Signed: index}

	// Test happy path
	updated, err := repo.updateComponentManifest("foo")
	assert.Nil(t, err)
	t.Logf("%+v", err)
	assert.NotNil(t, updated)
	assert.Contains(t, local.Saved, "foo.json")

	// Test that decrementing version numbers cause an error
	oldFoo := componentManifest()
	oldFoo.Version = 8
	local.Manifests["foo.json"] = &v1manifest.Manifest{Signed: oldFoo}
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
	someString := "foo201"
	mirror.Resources["/foo-2.0.1.tar.gz"] = someString
	local := v1manifest.NewMockManifests()
	setNewRoot(t, local)
	repo := NewV1Repo(&mirror, Options{}, local)
	item := versionItem()

	// Happy path file is as expected
	reader, err := repo.FetchComponent(&item)
	assert.Nil(t, err)
	buf := new(strings.Builder)
	_, err = io.Copy(buf, reader)
	assert.Nil(t, err)
	assert.Equal(t, someString, buf.String())

	// Sad paths

	// bad hash
	item.Hashes[v1manifest.SHA256] = "Not a hash"
	_, err = repo.FetchComponent(&item)
	assert.NotNil(t, err)

	//  Too long
	item.Length = 26
	_, err = repo.FetchComponent(&item)
	assert.NotNil(t, err)

	// missing tar ball/bad url
	item.URL = "/bar-2.0.1.tar.gz"
	_, err = repo.FetchComponent(&item)
	assert.NotNil(t, err)
}

func TestSelectVersion(t *testing.T) {
	mirror := MockMirror{
		Resources: map[string]string{},
	}
	local := v1manifest.NewMockManifests()
	setNewRoot(t, local)
	repo := NewV1Repo(&mirror, Options{}, local)

	// Simple case
	s, i, err := repo.selectVersion("foo", map[string]v1manifest.VersionItem{"v0.1.0": {URL: "1"}}, "")
	assert.Nil(t, err)
	assert.Equal(t, "v0.1.0", s)
	assert.Equal(t, "1", i.URL)

	// Choose by order
	s, i, err = repo.selectVersion("foo", map[string]v1manifest.VersionItem{"v0.1.0": {URL: "1"}, "v0.1.1": {URL: "2"}, "v0.2.0": {URL: "3"}}, "")
	assert.Nil(t, err)
	assert.Equal(t, "v0.2.0", s)
	assert.Equal(t, "3", i.URL)

	// Choose specific
	s, i, err = repo.selectVersion("foo", map[string]v1manifest.VersionItem{"v0.1.0": {URL: "1"}, "v0.1.1": {URL: "2"}, "v0.2.0": {URL: "3"}}, "v0.1.1")
	assert.Nil(t, err)
	assert.Equal(t, "v0.1.1", s)
	assert.Equal(t, "2", i.URL)

	// Target doesn't exists
	_, _, err = repo.selectVersion("foo", map[string]v1manifest.VersionItem{"v0.1.0": {URL: "1"}, "v0.1.1": {URL: "2"}, "v0.2.0": {URL: "3"}}, "v0.2.1")
	assert.NotNil(t, err)
}

func TestEnsureManifests(t *testing.T) {
	mirror := MockMirror{
		Resources: map[string]string{},
	}
	local := v1manifest.NewMockManifests()
	priv := setNewRoot(t, local)

	repo := NewV1Repo(&mirror, Options{}, local)

	index, _ := indexManifest(t)
	snapshot := snapshotManifest()
	snapStr := serialize(t, snapshot, priv)
	ts := timestampManifest()
	ts.Meta[v1manifest.ManifestURLSnapshot].Hashes[v1manifest.SHA256] = hash(snapStr)
	indexURL, _, _ := snapshot.VersionedURL(v1manifest.ManifestURLIndex)
	mirror.Resources[indexURL] = serialize(t, index, priv)
	mirror.Resources[v1manifest.ManifestURLSnapshot] = snapStr
	mirror.Resources[v1manifest.ManifestURLTimestamp] = serialize(t, ts, priv)

	// Initial repo
	err := repo.ensureManifests()
	assert.Nil(t, err)
	assert.Contains(t, local.Saved, v1manifest.ManifestFilenameIndex)
	assert.Contains(t, local.Saved, v1manifest.ManifestFilenameSnapshot)
	assert.Contains(t, local.Saved, v1manifest.ManifestFilenameTimestamp)
	assert.NotContains(t, local.Saved, v1manifest.ManifestFilenameRoot)

	// Happy update
	root2, priv2 := rootManifest(t)
	root, _ := repo.loadRoot()
	root2.Version = root.Version + 1
	mirror.Resources["/43.root.json"] = serialize(t, root2, priv, priv2)

	rootMeta := snapshot.Meta[v1manifest.ManifestURLRoot]
	rootMeta.Version = root2.Version
	snapshot.Meta[v1manifest.ManifestURLRoot] = rootMeta
	snapStr = serialize(t, snapshot, priv)
	ts.Meta[v1manifest.ManifestURLSnapshot].Hashes[v1manifest.SHA256] = hash(snapStr)
	ts.Version++
	mirror.Resources[v1manifest.ManifestURLSnapshot] = snapStr
	mirror.Resources[v1manifest.ManifestURLTimestamp] = serialize(t, ts, priv)
	local.Saved = []string{}

	err = repo.ensureManifests()
	assert.Nil(t, err)
	assert.Contains(t, local.Saved, v1manifest.ManifestFilenameRoot)

	// Sad path - root and snapshot disagree on version
	rootMeta.Version = 500
	snapshot.Meta[v1manifest.ManifestURLRoot] = rootMeta
	snapStr = serialize(t, snapshot, priv)
	ts.Meta[v1manifest.ManifestURLSnapshot].Hashes[v1manifest.SHA256] = hash(snapStr)
	ts.Version++
	mirror.Resources[v1manifest.ManifestURLSnapshot] = snapStr
	mirror.Resources[v1manifest.ManifestURLTimestamp] = serialize(t, ts, priv)

	err = repo.ensureManifests()
	assert.NotNil(t, err)
}

func TestUpdateComponents(t *testing.T) {
	mirror := MockMirror{
		Resources: map[string]string{},
	}
	local := v1manifest.NewMockManifests()
	priv := setNewRoot(t, local)

	repo := NewV1Repo(&mirror, Options{GOOS: "plat", GOARCH: "form"}, local)

	index, indexPriv := indexManifest(t)
	snapshot := snapshotManifest()
	snapStr := serialize(t, snapshot, priv)
	ts := timestampManifest()
	ts.Meta[v1manifest.ManifestURLSnapshot].Hashes[v1manifest.SHA256] = hash(snapStr)
	foo := componentManifest()
	indexURL, _, _ := snapshot.VersionedURL(v1manifest.ManifestURLIndex)
	mirror.Resources[indexURL] = serialize(t, index, priv)
	mirror.Resources[v1manifest.ManifestURLSnapshot] = snapStr
	mirror.Resources[v1manifest.ManifestURLTimestamp] = serialize(t, ts, priv)
	mirror.Resources["/7.foo.json"] = serialize(t, foo, indexPriv)
	mirror.Resources["/foo-2.0.1.tar.gz"] = "foo201"

	// Install
	err := repo.UpdateComponents([]ComponentSpec{{
		ID: "foo",
	}})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(local.Installed))
	assert.Equal(t, "v2.0.1", local.Installed["foo"].Version)
	assert.Equal(t, "foo201", local.Installed["foo"].Contents)

	// Update
	foo.Version = 8
	foo.Platforms["plat/form"]["v2.0.2"] = versionItem2()
	mirror.Resources["/8.foo.json"] = serialize(t, foo, indexPriv)
	mirror.Resources["/foo-2.0.2.tar.gz"] = "foo202"
	snapshot.Version++
	item := snapshot.Meta["/foo.json"]
	item.Version = 8
	snapshot.Meta["/foo.json"] = item
	snapStr = serialize(t, snapshot, priv)
	ts.Meta[v1manifest.ManifestURLSnapshot].Hashes[v1manifest.SHA256] = hash(snapStr)
	ts.Version++
	mirror.Resources[v1manifest.ManifestURLSnapshot] = snapStr
	mirror.Resources[v1manifest.ManifestURLTimestamp] = serialize(t, ts, priv)
	err = repo.UpdateComponents([]ComponentSpec{{
		ID: "foo",
	}})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(local.Installed))
	assert.Equal(t, "v2.0.2", local.Installed["foo"].Version)
	assert.Equal(t, "foo202", local.Installed["foo"].Contents)

	// Update; already up to date
	err = repo.UpdateComponents([]ComponentSpec{{
		ID: "foo",
	}})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(local.Installed))
	assert.Equal(t, "v2.0.2", local.Installed["foo"].Version)
	assert.Equal(t, "foo202", local.Installed["foo"].Contents)

	// Specific version
	err = repo.UpdateComponents([]ComponentSpec{{
		ID:      "foo",
		Version: "v2.0.1",
	}})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(local.Installed))
	assert.Equal(t, "v2.0.1", local.Installed["foo"].Version)
	assert.Equal(t, "foo201", local.Installed["foo"].Contents)

	// Sad paths
	// Component doesn't exists
	err = repo.UpdateComponents([]ComponentSpec{{
		ID: "bar",
	}})
	assert.Nil(t, err)
	_, ok := local.Installed["bar"]
	assert.False(t, ok)

	// Specific version doesn't exist
	err = repo.UpdateComponents([]ComponentSpec{{
		ID:      "foo",
		Version: "v2.0.3",
	}})
	assert.NotNil(t, err)
	assert.Equal(t, 1, len(local.Installed))
	assert.Equal(t, "v2.0.1", local.Installed["foo"].Version)
	assert.Equal(t, "foo201", local.Installed["foo"].Contents)

	// Platform not supported
	repo.GOARCH = "sdfsadfsadf"
	err = repo.UpdateComponents([]ComponentSpec{{
		ID: "foo",
	}})
	assert.NotNil(t, err)
	assert.Equal(t, 1, len(local.Installed))
	assert.Equal(t, "v2.0.1", local.Installed["foo"].Version)
	assert.Equal(t, "foo201", local.Installed["foo"].Contents)

	// Already installed
	repo.GOARCH = "form"
	err = repo.UpdateComponents([]ComponentSpec{{
		ID:      "foo",
		Version: "v2.0.1",
	}})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(local.Installed))
	assert.Equal(t, "v2.0.1", local.Installed["foo"].Version)
	assert.Equal(t, "foo201", local.Installed["foo"].Contents)

	// Test that even after one error, other components are handled
	repo.GOARCH = "form"
	err = repo.UpdateComponents([]ComponentSpec{{
		ID: "bar",
	}, {
		ID: "foo",
	}})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(local.Installed))
	assert.Equal(t, "v2.0.2", local.Installed["foo"].Version)
	assert.Equal(t, "foo202", local.Installed["foo"].Contents)
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
			v1manifest.ManifestURLRoot:  {Version: 42},
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
		ID:          "foo",
		Description: "foo does stuff",
		Platforms: map[string]map[string]v1manifest.VersionItem{
			"plat/form": {"v2.0.1": versionItem()},
		},
	}
}

func versionItem() v1manifest.VersionItem {
	return v1manifest.VersionItem{
		URL: "/foo-2.0.1.tar.gz",
		FileHash: v1manifest.FileHash{
			Hashes: map[string]string{v1manifest.SHA256: "8dc7102c0d675dfa53da273317b9f627e96ed24efeecc8c5ebd00dc06f4e09c3"},
			Length: 28,
		},
	}
}

func versionItem2() v1manifest.VersionItem {
	return v1manifest.VersionItem{
		URL: "/foo-2.0.2.tar.gz",
		FileHash: v1manifest.FileHash{
			Hashes: map[string]string{v1manifest.SHA256: "5abe91bc22039c15c05580062357be7ab0bfd7968582a118fbb4eb817ddc2e76"},
			Length: 12,
		},
	}
}

func indexManifest(t *testing.T) (*v1manifest.Index, crypto.PrivKey) {
	info, keyID, priv, err := v1manifest.FreshKeyInfo()
	assert.Nil(t, err)
	bytes, err := priv.Serialize()
	assert.Nil(t, err)
	privKeyInfo := v1manifest.NewKeyInfo(bytes)
	// The signed id will be priveID and it should be equal as keyID
	privID, err := privKeyInfo.ID()
	assert.Nil(t, err)
	assert.Equal(t, keyID, privID)

	return &v1manifest.Index{
		SignedBase: v1manifest.SignedBase{
			Ty:          v1manifest.ManifestTypeIndex,
			SpecVersion: "0.1.0",
			Expires:     "2220-05-11T04:51:08Z",
			Version:     5,
		},
		Owners: map[string]v1manifest.Owner{"bar": {
			Name:      "Bar",
			Keys:      map[string]*v1manifest.KeyInfo{keyID: info},
			Threshold: 1,
		}},
		Components: map[string]v1manifest.ComponentItem{"foo": {
			Yanked: false,
			Owner:  "bar",
			URL:    "/foo.json",
		}},
		DefaultComponents: []string{},
	}, priv
}

func rootManifest(t *testing.T) (*v1manifest.Root, crypto.PrivKey) {
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
			v1manifest.ManifestTypeTimestamp: {
				URL:       v1manifest.ManifestURLTimestamp,
				Keys:      map[string]*v1manifest.KeyInfo{keyID: info},
				Threshold: 1,
			},
			v1manifest.ManifestTypeSnapshot: {
				URL:       v1manifest.ManifestURLSnapshot,
				Keys:      map[string]*v1manifest.KeyInfo{keyID: info},
				Threshold: 1,
			},
		},
	}, priv
}

func setNewRoot(t *testing.T, local *v1manifest.MockManifests) crypto.PrivKey {
	root, privk := rootManifest(t)
	setRoot(local, root)
	return privk
}

func setRoot(local *v1manifest.MockManifests, root *v1manifest.Root) {
	local.Manifests[v1manifest.ManifestFilenameRoot] = &v1manifest.Manifest{Signed: root}
	for r, ks := range root.Roles {
		_ = local.Ks.AddKeys(r, 1, "2220-05-11T04:51:08Z", ks.Keys)
	}
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

func hash(s string) string {
	shaWriter := sha256.New()
	if _, err := io.Copy(shaWriter, strings.NewReader(s)); err != nil {
		panic(err)
	}

	return hex.EncodeToString(shaWriter.Sum(nil))
}

// Test we can correctly load manifests generate by tools/migrate
// which generate the v1manifest from the v0manifest.
//func TestWithMigrate(t *testing.T) {
//	// generate using tools/migrate
//	mdir := "./testdata/manifests"
//
//	// create a repo using the manifests as a mirror.
//	// profileDir will contains the only trusted root.
//	repo, profileDir := createMigrateRepo(t, mdir)
//	_, err := repo.loadRoot()
//	assert.Nil(t, err)
//	defer os.RemoveAll(profileDir)
//	_ = profileDir
//
//	err = repo.updateLocalRoot()
//	assert.Nil(t, err)
//
//	_, err = repo.ensureManifests()
//	assert.Nil(t, err)
//
//	// after ensureManifests we should can load index/timestamp/snapshot
//	var snap v1manifest.Snapshot
//	var index v1manifest.Index
//	var root v1manifest.Root
//	{
//		exists, err := repo.local.LoadManifest(&index)
//		assert.Nil(t, err)
//		assert.True(t, exists)
//		exists, err = repo.local.LoadManifest(&root)
//		assert.Nil(t, err)
//		assert.True(t, exists)
//		exists, err = repo.local.LoadManifest(&snap)
//		assert.Nil(t, err)
//		assert.True(t, exists)
//	}
//
//	// check can load component manifests.
//	assert.NotZero(t, len(snap.Meta))
//	assert.NotZero(t, len(index.Components))
//	{
//		// Every component should in snapshot's meta
//		t.Logf("snap meta: %+v", snap.Meta)
//		for _, item := range index.Components {
//			_, ok := snap.Meta[item.URL]
//			assert.True(t, ok, "component url: %s", item.URL)
//		}
//
//		// Test after updateComponentManifest we can load it locally.
//		for id, item := range index.Components {
//			_, err := repo.updateComponentManifest(id)
//			assert.Nil(t, err)
//
//			filename := v1manifest.ComponentManifestFilename(id)
//			_, _, err = snap.VersionedURL(item.URL)
//			assert.Nil(t, err)
//			_, err = repo.local.LoadComponentManifest(&index, filename)
//			assert.Nil(t, err)
//		}
//	}
//}

/*
func createMigrateRepo(t *testing.T, mdir string) (repo *V1Repository, profileDir string) {
	var err error
	profileDir, err = ioutil.TempDir("", "tiup-*")
	assert.Nil(t, err)
	t.Logf("using profile dir: %s", profileDir)

	// copy root.json from mdir to profileDir
	data, err := ioutil.ReadFile(filepath.Join(mdir, "root.json"))
	assert.Nil(t, err)
	err = ioutil.WriteFile(filepath.Join(profileDir, "root.json"), data, 0644)
	assert.Nil(t, err)

	localdata.DefaultTiupHome = profileDir
	profile := localdata.InitProfile()
	options := Options{}
	mirror := NewMirror(mdir, MirrorOptions{})
	local, err := v1manifest.NewManifests(profile)
	assert.Nil(t, err)
	repo = NewV1Repo(mirror, options, local)
	return
}
*/
