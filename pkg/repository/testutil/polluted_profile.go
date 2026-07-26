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

package testutil

import (
	"os"
	"path/filepath"
	"time"

	cjson "github.com/gibson042/canonicaljson-go"
	"github.com/pingcap/tiup/pkg/crypto"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
)

const (
	componentID = "tidb"
	ownerName   = "pingcap"
)

var stableInitTime = time.Date(2219, 5, 11, 4, 51, 8, 0, time.UTC)

// PollutedProfileFixture mirrors the old polluted test profile while keeping it
// free from hard-coded expiry dates that eventually go stale.
type PollutedProfileFixture struct {
	Dir                    string
	RemoteExpiredTimestamp string
	RemoteTimestamp        string
	RemoteIndex            string
}

// NewPollutedProfileFixture creates a profile directory whose local manifests
// intentionally mix valid, expired, and invalid signatures.
func NewPollutedProfileFixture() (*PollutedProfileFixture, error) {
	dir, err := os.MkdirTemp("", "tiup-polluted-*")
	if err != nil {
		return nil, err
	}

	cleanupOnError := func(err error) (*PollutedProfileFixture, error) {
		_ = os.RemoveAll(dir)
		return nil, err
	}

	rootPrivKeys, rootPubKeys, err := newKeyInfos(3)
	if err != nil {
		return cleanupOnError(err)
	}

	indexPriv, indexPub, err := newKeyInfo()
	if err != nil {
		return cleanupOnError(err)
	}
	snapshotPriv, snapshotPub, err := newKeyInfo()
	if err != nil {
		return cleanupOnError(err)
	}
	timestampPriv, timestampPub, err := newKeyInfo()
	if err != nil {
		return cleanupOnError(err)
	}
	ownerPriv, ownerPub, err := newKeyInfo()
	if err != nil {
		return cleanupOnError(err)
	}
	badIndexPriv, _, err := newKeyInfo()
	if err != nil {
		return cleanupOnError(err)
	}

	root := v1manifest.NewRoot(stableInitTime)
	root.Version = 1
	if err := root.SetRole(root, rootPubKeys...); err != nil {
		return cleanupOnError(err)
	}

	index := v1manifest.NewIndex(stableInitTime)
	index.Version = 420
	ownerKeyID, err := ownerPub.ID()
	if err != nil {
		return cleanupOnError(err)
	}
	index.Owners = map[string]v1manifest.Owner{
		ownerName: {
			Name:      "PingCAP",
			Threshold: 1,
			Keys: map[string]*v1manifest.KeyInfo{
				ownerKeyID: ownerPub,
			},
		},
	}
	index.Components = map[string]v1manifest.ComponentItem{
		componentID: {
			Owner: ownerName,
			URL:   "/" + componentID + ".json",
		},
	}
	if err := root.SetRole(index, indexPub); err != nil {
		return cleanupOnError(err)
	}

	snapshot := v1manifest.NewSnapshot(stableInitTime)
	snapshot.Version = 42
	if err := root.SetRole(snapshot, snapshotPub); err != nil {
		return cleanupOnError(err)
	}

	timestamp := v1manifest.NewTimestamp(stableInitTime)
	timestamp.Version = 639
	if err := root.SetRole(timestamp, timestampPub); err != nil {
		return cleanupOnError(err)
	}

	component := v1manifest.NewComponent(componentID, "TiDB", stableInitTime)
	component.Version = 62
	component.Platforms = map[string]map[string]v1manifest.VersionItem{}

	signedRoot, rootContent, err := signManifest(root, rootPrivKeys...)
	if err != nil {
		return cleanupOnError(err)
	}
	signedIndex, remoteIndex, err := signManifest(index, indexPriv)
	if err != nil {
		return cleanupOnError(err)
	}
	signedComponent, componentContent, err := signManifest(component, ownerPriv)
	if err != nil {
		return cleanupOnError(err)
	}

	if _, err := snapshot.SetVersions(map[string]*v1manifest.Manifest{
		"root":      signedRoot,
		"index":     signedIndex,
		componentID: signedComponent,
	}); err != nil {
		return cleanupOnError(err)
	}
	signedSnapshot, snapshotContent, err := signManifest(snapshot, snapshotPriv)
	if err != nil {
		return cleanupOnError(err)
	}

	localTimestamp := v1manifest.NewTimestamp(stableInitTime)
	localTimestamp.Version = 638
	localTimestamp.Expires = "2000-08-01T14:47:48+08:00"
	if _, err := localTimestamp.SetSnapshot(signedSnapshot); err != nil {
		return cleanupOnError(err)
	}
	_, localTimestampContent, err := signManifest(localTimestamp, timestampPriv)
	if err != nil {
		return cleanupOnError(err)
	}

	remoteExpiredTimestamp := v1manifest.NewTimestamp(stableInitTime)
	remoteExpiredTimestamp.Version = 639
	remoteExpiredTimestamp.Expires = "2000-08-01T14:47:48+08:00"
	if _, err := remoteExpiredTimestamp.SetSnapshot(signedSnapshot); err != nil {
		return cleanupOnError(err)
	}
	_, remoteExpiredTimestampContent, err := signManifest(remoteExpiredTimestamp, timestampPriv)
	if err != nil {
		return cleanupOnError(err)
	}

	remoteTimestamp := v1manifest.NewTimestamp(stableInitTime)
	remoteTimestamp.Version = 99999
	if _, err := remoteTimestamp.SetSnapshot(signedSnapshot); err != nil {
		return cleanupOnError(err)
	}
	_, remoteTimestampContent, err := signManifest(remoteTimestamp, timestampPriv)
	if err != nil {
		return cleanupOnError(err)
	}

	_, localBadIndexContent, err := signManifest(index, badIndexPriv)
	if err != nil {
		return cleanupOnError(err)
	}

	for _, dirName := range []string{
		filepath.Join(dir, "bin"),
		filepath.Join(dir, "manifests"),
	} {
		if err := os.MkdirAll(dirName, 0o755); err != nil {
			return cleanupOnError(err)
		}
	}

	writes := map[string]string{
		filepath.Join(dir, "bin", v1manifest.ManifestFilenameRoot):                    rootContent,
		filepath.Join(dir, "manifests", v1manifest.ManifestFilenameSnapshot):          snapshotContent,
		filepath.Join(dir, "manifests", v1manifest.ManifestFilenameIndex):             localBadIndexContent,
		filepath.Join(dir, "manifests", v1manifest.ManifestFilenameTimestamp):         localTimestampContent,
		filepath.Join(dir, "manifests", v1manifest.ComponentManifestFilename("tidb")): componentContent,
	}
	for name, content := range writes {
		if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
			return cleanupOnError(err)
		}
	}

	return &PollutedProfileFixture{
		Dir:                    dir,
		RemoteExpiredTimestamp: remoteExpiredTimestampContent,
		RemoteTimestamp:        remoteTimestampContent,
		RemoteIndex:            remoteIndex,
	}, nil
}

func newKeyInfos(count int) ([]*v1manifest.KeyInfo, []*v1manifest.KeyInfo, error) {
	privs := make([]*v1manifest.KeyInfo, 0, count)
	pubs := make([]*v1manifest.KeyInfo, 0, count)
	for range count {
		priv, pub, err := newKeyInfo()
		if err != nil {
			return nil, nil, err
		}
		privs = append(privs, priv)
		pubs = append(pubs, pub)
	}
	return privs, pubs, nil
}

func newKeyInfo() (*v1manifest.KeyInfo, *v1manifest.KeyInfo, error) {
	priv, err := crypto.NewKeyPair(crypto.KeyTypeRSA, crypto.KeySchemeRSASSAPSSSHA256)
	if err != nil {
		return nil, nil, err
	}
	serializedPriv, err := priv.Serialize()
	if err != nil {
		return nil, nil, err
	}
	privInfo := v1manifest.NewKeyInfo(serializedPriv)
	pubInfo, err := privInfo.Public()
	if err != nil {
		return nil, nil, err
	}
	return privInfo, pubInfo, nil
}

func signManifest(role v1manifest.ValidManifest, keys ...*v1manifest.KeyInfo) (*v1manifest.Manifest, string, error) {
	manifest, err := v1manifest.SignManifest(role, keys...)
	if err != nil {
		return nil, "", err
	}
	content, err := cjson.Marshal(manifest)
	if err != nil {
		return nil, "", err
	}
	return manifest, string(content), nil
}
