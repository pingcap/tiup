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
	"os"
	"path"
	"strings"
	"testing"

	cjson "github.com/gibson042/canonicaljson-go"
	"github.com/google/uuid"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/crypto"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/stretchr/testify/assert"
)

// Create a profile directory
// Contained stuff:
//   - index.json: with wrong signature
//   - snapshot.json: correct
//   - tidb.json: correct
//   - timestamp: with expired timestamp
func genPollutedProfileDir() (string, error) {
	uid := uuid.New().String()
	dir := path.Join("/tmp", uid)

	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	if err := utils.Copy(path.Join(wd, "testdata", "polluted"), dir); err != nil {
		return "", err
	}

	return dir, nil
}

func TestPollutedManifest(t *testing.T) {
	profileDir, err := genPollutedProfileDir()
	assert.Nil(t, err)
	defer os.RemoveAll(profileDir)

	profile := localdata.NewProfile(profileDir, "todo", &localdata.TiUPConfig{})
	local, err := v1manifest.NewManifests(profile)
	assert.Nil(t, err)

	// Mock remote expired
	mirror := MockMirror{
		Resources: map[string]string{
			"/timestamp.json": `{"signatures":[{"keyid":"66d4ea1da00076c822a6e1b4df5eb1e529eb38f6edcedff323e62f2bfe3eaddd","sig":"V8MgDDCmfVb8N0O3unbAno8q6i2Ag1Sbr/3n12Odk8McKzZaif7OcDm1IZB5J3o7ajsBF1tduTrcO7OijJQvx8l9i6aZi9J1lb/eJpYsyvQWdzd/T7osdRkEIhtM4/sGFjGslOolTFmpA/U5IkJ+FWAi38YaFPRn8bfIPLGniRAYs4/qjLBB3RgBUlDIIVvTiJIHEHtf3Bqb5LjpEjW4XhmDK94LJbKUqfO/6oDnQzI6Rot7zBWwDQVrIHakvQxoqA5c2jtMHCXSdX9cN7aRrNO4csggMzvQot7K0JYYszlroXnsL2ioNMgcPhtoEaMLW9mFjmdgR0j1//n1mxtdWA=="}],"signed":{"_type":"timestamp","expires":"2000-08-01T14:47:48+08:00","meta":{"/snapshot.json":{"hashes":{"sha256":"24c9fa83f15eda0683999b98ac0ff87fb95aed91c10410891fb38313f38e35c1"},"length":1760}},"spec_version":"0.1.0","version":639}}`,
		},
	}
	repo := NewV1Repo(&mirror, Options{}, local)
	err = repo.UpdateComponentManifests()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "has expired at")

	// Mock remote correct but local throw sign error
	mirror = MockMirror{
		Resources: map[string]string{
			// these files will expire after 2120
			"/timestamp.json": `{"signatures":[{"keyid":"66d4ea1da00076c822a6e1b4df5eb1e529eb38f6edcedff323e62f2bfe3eaddd","sig":"Mo/o68zmRCpu5gHB9uEyVVtf5Cz432F6dd/jkKVDQXw3vG4ftnOiIRF590OfE79VFQ3BXDgMUmfD7sdkcU5Gc4HBUqKWt3vI1tFsEFfxZDQSA/upONirknxtKKZtrjDkk/rjD7cLE3S/Stul+ho0LwwlFncUZmdYaaXBeP9YileekUR15S+XvIInNO5YK+vy2EqTjMQdYTMabhZHxvP35MUMnphXuBV0aLjuvkaksF2V0mXFGcB9GUHzmgGuW32VoF2G6UBiRYaSu0r1LjTqS2auL/TciLZmi95KAAlCz3f8SOpkQ0z5bZiSk8SFCbPN1tb97RSaFJQ1jnEmxZULZA=="}],"signed":{"_type":"timestamp","expires":"2120-08-01T14:47:48+08:00","meta":{"/snapshot.json":{"hashes":{"sha256":"acb8b69c0335c6b4f3b0315dc58bdf774fca558c050dd16ae4459e3a82d39261"},"length":1760}},"spec_version":"0.1.0","version":99999}}`,
			"/420.index.json": `{"signatures":[{"keyid":"7fce7ec4f9c36d51dec7ec96065bb64958b743e46ea8141da668cd2ce58a9e61","sig":"MFVJxmU1ErcobtRcssvQiHQLTAVyCVieVpSNlgIAv6FSGN8utFpKuV3RKgelgJskt2j/gJ6k+ITpUqd1Y9iiff44N9u9oov0CBR7gYoSfSum8yEqI0AeJKePUu40959xe/1WU881npF50+LE8ovk0MC8mXzNDoe4kHWFZBve9s+VbPS1KwD2jdnf9sR95rJLF8vxMwnDwsXmdf5Y4TV9nUvQ996BRmr0YFQKBVl9DqxQ+Y2KyXjrNaZwJhKdF7I3W6fCOF4Cf5QMbZ6SG4TrPsscjscZQKJHox3iMuL6NGEem2B2lEJovrlBh0U9/cN4y6CZvEnL3uGYp7gxVvTe7A=="}],"signed":{"_type":"index","components":{"tidb":{"hidden":false,"owner":"pingcap","standalone":false,"url":"/tidb.json","yanked":false}},"default_components":[],"expires":"2121-06-23T12:05:15+08:00","owners":{"pingcap":{"keys":{"a61b695e2b86097d993e94e99fd15ec6d8fc8e9522948c9ff21c2f2c881093ae":{"keytype":"rsa","keyval":{"public":"-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAnayxhw6KeoKK+Ax9RW6v\n66YjrpRpGLewLmSSAzJGX8nL5/a2nEbXbeF9po265KcBSFWol8jLBsmG56ruwwxp\noWWhJPncqGqy8wMeRMmTf7ATGa+tk+To7UAQD0MYzt7rRlIdpqi9Us3J6076Z83k\n2sxFnX9sVflhOsotGWL7hmrn/CJWxKsO6OVCoqbIlnJV8xFazE2eCfaDTIEEEgnh\nLIGDsmv1AN8ImUIn/hyKcm1PfhDZrF5qhEVhfz5D8aX3cUcEJw8BvCaNloXyHf+y\nDKjqO/dJ7YFWVt7nPqOvaEkBQGMd54ETJ/BbO9r3WTsjXKleoPovBSQ/oOxApypb\nNQIDAQAB\n-----END PUBLIC KEY-----\n"},"scheme":"rsassa-pss-sha256"}},"name":"PingCAP","threshold":1}},"spec_version":"0.1.0","version":420}}`,
		},
	}
	repo = NewV1Repo(&mirror, Options{}, local)
	err = repo.UpdateComponentManifests()
	assert.Nil(t, err)
}

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
	repo.PurgeTimestamp()
	_, _, err = repo.fetchTimestamp()
	assert.NotNil(t, err)

	// Test that an invalid manifest from the mirror causes an error
	invalidTimestamp := timestampManifest()
	invalidTimestamp.SpecVersion = "10.1.0"
	_, _, err = repo.fetchTimestamp()
	assert.NotNil(t, err)

	// TODO test that a bad signature causes an error
}

func TestCacheTimestamp(t *testing.T) {
	mirror := MockMirror{
		Resources: map[string]string{},
	}
	local := v1manifest.NewMockManifests()
	privk := setNewRoot(t, local)
	repo := NewV1Repo(&mirror, Options{}, local)

	repoTimestamp := timestampManifest()
	mirror.Resources[v1manifest.ManifestURLTimestamp] = serialize(t, repoTimestamp, privk)
	changed, _, err := repo.fetchTimestamp()
	assert.Nil(t, err)
	assert.True(t, changed)

	delete(mirror.Resources, v1manifest.ManifestURLTimestamp)
	changed, _, err = repo.fetchTimestamp()
	assert.Nil(t, err)
	assert.False(t, changed)

	repo.PurgeTimestamp()
	_, _, err = repo.fetchTimestamp()
	assert.NotNil(t, err)
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

func TestYanked(t *testing.T) {
	mirror := MockMirror{
		Resources: map[string]string{},
	}
	local := v1manifest.NewMockManifests()
	_ = setNewRoot(t, local)
	repo := NewV1Repo(&mirror, Options{}, local)

	index, indexPriv := indexManifest(t)
	snapshot := snapshotManifest()
	bar := componentManifest()
	serBar := serialize(t, bar, indexPriv)
	mirror.Resources["/7.bar.json"] = serBar
	local.Manifests[v1manifest.ManifestFilenameSnapshot] = &v1manifest.Manifest{Signed: snapshot}
	local.Manifests[v1manifest.ManifestFilenameIndex] = &v1manifest.Manifest{Signed: index}

	// Test yanked version
	updated, err := repo.updateComponentManifest("bar", true)
	assert.Nil(t, err)
	_, ok := updated.VersionList("plat/form")["v2.0.3"]
	assert.False(t, ok)

	_, err = repo.updateComponentManifest("bar", false)
	assert.NotNil(t, err)
	assert.Equal(t, errors.Cause(err), ErrUnknownComponent)
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
	updated, err := repo.updateComponentManifest("foo", false)
	assert.Nil(t, err)
	t.Logf("%+v", err)
	assert.NotNil(t, updated)
	assert.Contains(t, local.Saved, "foo.json")

	// Test that decrementing version numbers cause an error
	oldFoo := componentManifest()
	oldFoo.Version = 8
	local.Manifests["foo.json"] = &v1manifest.Manifest{Signed: oldFoo}
	local.Saved = []string{}
	updated, err = repo.updateComponentManifest("foo", false)
	assert.NotNil(t, err)
	assert.Nil(t, updated)
	assert.Empty(t, local.Saved)

	// Test that id missing from index causes an error
	updated, err = repo.updateComponentManifest("bar", false)
	assert.NotNil(t, err)
	assert.Nil(t, updated)
	assert.Empty(t, local.Saved)

	// TODO check that the correct files were created
	// TODO test that invalid signature of component manifest causes an error
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
	root2, priv2 := rootManifest(t) // generate new root key
	root, _ := repo.loadRoot()
	root2.Version = root.Version + 1
	mirror.Resources["/43.root.json"] = serialize(t, root2, priv, priv2)

	rootMeta := snapshot.Meta[v1manifest.ManifestURLRoot]
	rootMeta.Version = root2.Version
	snapshot.Meta[v1manifest.ManifestURLRoot] = rootMeta
	snapStr = serialize(t, snapshot, priv2) // sign snapshot with new key
	ts.Meta[v1manifest.ManifestURLSnapshot].Hashes[v1manifest.SHA256] = hash(snapStr)
	ts.Version++
	mirror.Resources[v1manifest.ManifestURLSnapshot] = snapStr
	mirror.Resources[v1manifest.ManifestURLTimestamp] = serialize(t, ts, priv2)
	local.Saved = []string{}

	repo.PurgeTimestamp()
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

	repo.PurgeTimestamp()
	err = repo.ensureManifests()
	assert.NotNil(t, err)
}

func TestLatestStableVersion(t *testing.T) {
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
	// v2.0.1: unyanked
	// v2.0.3: yanked
	// v3.0.0-rc: unyanked
	foo := componentManifest()
	indexURL, _, _ := snapshot.VersionedURL(v1manifest.ManifestURLIndex)
	mirror.Resources[indexURL] = serialize(t, index, priv)
	mirror.Resources[v1manifest.ManifestURLSnapshot] = snapStr
	mirror.Resources[v1manifest.ManifestURLTimestamp] = serialize(t, ts, priv)
	mirror.Resources["/7.foo.json"] = serialize(t, foo, indexPriv)

	v, _, err := repo.LatestStableVersion("foo", false)
	assert.Nil(t, err)
	assert.Equal(t, "v2.0.1", v.String())

	v, _, err = repo.LatestStableVersion("foo", true)
	assert.Nil(t, err)
	assert.Equal(t, "v2.0.3", v.String())
}

func TestLatestStableVersionWithPrerelease(t *testing.T) {
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

	// v2.0.1: yanked
	// v2.0.3: yanked
	// v3.0.0-rc: unyanked
	item := foo.Platforms["plat/form"]["v2.0.1"]
	item.Yanked = true
	foo.Platforms["plat/form"]["v2.0.1"] = item

	indexURL, _, _ := snapshot.VersionedURL(v1manifest.ManifestURLIndex)
	mirror.Resources[indexURL] = serialize(t, index, priv)
	mirror.Resources[v1manifest.ManifestURLSnapshot] = snapStr
	mirror.Resources[v1manifest.ManifestURLTimestamp] = serialize(t, ts, priv)
	mirror.Resources["/7.foo.json"] = serialize(t, foo, indexPriv)

	v, _, err := repo.LatestStableVersion("foo", false)
	assert.Nil(t, err)
	assert.Equal(t, "v3.0.0-rc", v.String())
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
	mirror.Resources["/foo-3.0.0-rc.tar.gz"] = "foo300rc"

	// Install
	err := repo.UpdateComponents([]ComponentSpec{{
		ID: "foo",
	}})

	assert.Nil(t, err)
	assert.Equal(t, 1, len(local.Installed))
	assert.Equal(t, "v2.0.1", local.Installed["foo"].Version)
	assert.Equal(t, "foo201", local.Installed["foo"].Contents)
	assert.Equal(t, "/tmp/mock/components/foo/v2.0.1/foo-2.0.1.tar.gz", local.Installed["foo"].BinaryPath)

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
	repo.PurgeTimestamp()
	err = repo.UpdateComponents([]ComponentSpec{{
		ID:        "foo",
		TargetDir: "/tmp/mock-mock",
	}})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(local.Installed))
	assert.Equal(t, "v2.0.2", local.Installed["foo"].Version)
	assert.Equal(t, "foo202", local.Installed["foo"].Contents)
	assert.Equal(t, "/tmp/mock-mock/foo-2.0.2.tar.gz", local.Installed["foo"].BinaryPath)

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

	// Install preprelease version
	// Specific version
	err = repo.UpdateComponents([]ComponentSpec{{
		ID:      "foo",
		Version: "v3.0.0-rc",
	}})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(local.Installed))
	assert.Equal(t, "v3.0.0-rc", local.Installed["foo"].Version)
	assert.Equal(t, "foo300rc", local.Installed["foo"].Contents)
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
			"/bar.json":                 {Version: 7},
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
			"plat/form": {
				"v2.0.1":    versionItem(),
				"v2.0.3":    versionItem3(),
				"v3.0.0-rc": versionItemPrerelease(),
			},
		},
	}
}

func versionItemPrerelease() v1manifest.VersionItem {
	return v1manifest.VersionItem{
		URL:   "/foo-3.0.0-rc.tar.gz",
		Entry: "dummy",
		FileHash: v1manifest.FileHash{
			Hashes: map[string]string{v1manifest.SHA256: "0cd2f56431d966c8897c87193539aabb3ffb34b1c55aad4b8a03dd6421cec5aa"},
			Length: 28,
		},
	}
}

func versionItem() v1manifest.VersionItem {
	return v1manifest.VersionItem{
		URL:   "/foo-2.0.1.tar.gz",
		Entry: "dummy",
		FileHash: v1manifest.FileHash{
			Hashes: map[string]string{v1manifest.SHA256: "8dc7102c0d675dfa53da273317b9f627e96ed24efeecc8c5ebd00dc06f4e09c3"},
			Length: 28,
		},
	}
}

func versionItem2() v1manifest.VersionItem {
	return v1manifest.VersionItem{
		URL:   "/foo-2.0.2.tar.gz",
		Entry: "dummy",
		FileHash: v1manifest.FileHash{
			Hashes: map[string]string{v1manifest.SHA256: "5abe91bc22039c15c05580062357be7ab0bfd7968582a118fbb4eb817ddc2e76"},
			Length: 12,
		},
	}
}

func versionItem3() v1manifest.VersionItem {
	return v1manifest.VersionItem{
		URL:    "/foo-2.0.3.tar.gz",
		Entry:  "dummy",
		Yanked: true,
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
		Components: map[string]v1manifest.ComponentItem{
			"foo": {
				Yanked: false,
				Owner:  "bar",
				URL:    "/foo.json",
			},
			"bar": {
				Yanked: true,
				Owner:  "bar",
				URL:    "/bar.json",
			},
		},
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
		priv, err = crypto.NewKeyPair(crypto.KeyTypeRSA, crypto.KeySchemeRSASSAPSSSHA256)
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
