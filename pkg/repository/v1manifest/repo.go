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

package v1manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	cjson "github.com/gibson042/canonicaljson-go"
	"github.com/pingcap-incubator/tiup/pkg/repository/crypto"
	"github.com/pingcap/errors"
)

// Init creates and initializes an empty repository
func Init(dst string, initTime time.Time, priv string) error {
	// read key files
	privBytes, err := ioutil.ReadFile(priv)
	if err != nil {
		return err
	}
	privKey := &crypto.RSAPrivKey{}
	if err = privKey.Deserialize(privBytes); err != nil {
		return err
	}

	// initial manifests
	manifests := make(map[string]ValidManifest)

	// init the root manifest
	manifests[ManifestTypeRoot] = NewRoot(initTime)

	// init index
	manifests[ManifestTypeIndex] = NewIndex(initTime)

	// snapshot and timestamp are the last two manifests to be initialized
	// init snapshot
	manifests[ManifestTypeSnapshot] = NewSnapshot(initTime).SetVersions(manifests)

	// init timestamp
	timestamp, err := NewTimestamp(initTime).SetSnapshot(manifests[ManifestTypeSnapshot].(*Snapshot))
	manifests[ManifestTypeTimestamp] = NewTimestamp(initTime)
	if err != nil {
		return err
	}
	manifests[ManifestTypeTimestamp] = timestamp

	// root and snapshot has meta of each other inside themselves, but it's ok here
	// as we are still during the init process, not version bump needed
	for ty, val := range ManifestsConfig {
		if val.Filename == "" {
			// skip unsupported ManifestsConfig such as component
			continue
		}
		if m, ok := manifests[ty]; ok {
			manifests[ManifestTypeRoot].(*Root).SetRole(m)
			continue
		}
		// FIXME: log a warning about manifest not found instead of returning error
		return fmt.Errorf("manifest '%s' not initialized porperly", ty)
	}

	return BatchSaveManifests(dst, manifests, privKey)
}

// AddComponent adds a new component to an existing repository
func AddComponent(id, name, desc, owner, repo string, isDefault bool, pub, priv string) error {
	id = strings.ToLower(id)

	// read key files
	privBytes, err := ioutil.ReadFile(priv)
	if err != nil {
		return err
	}
	privKey := &crypto.RSAPrivKey{}
	if err = privKey.Deserialize(privBytes); err != nil {
		return err
	}

	// read manifest files from disk
	manifests, err := ReadManifestDir(repo)
	if err != nil {
		return nil
	}

	// check id conflicts
	if _, found := ManifestsConfig[id]; found {
		// reserved keywords
		return fmt.Errorf("component id '%s' is not allowed, please use another one", id)
	}
	if _, found := manifests[ManifestTypeIndex].(*Index).Components[id]; found {
		return fmt.Errorf("component id '%s' already exist, please use another one", id)
	}

	// create new component manifest
	currTime := time.Now().UTC()
	comp := NewComponent(id, name, desc, currTime)
	manifests[id] = comp

	// update repository
	compInfo := ComponentItem{
		Owner:     owner,
		URL:       comp.Filename(),
		Threshold: 1,
	}
	index := manifests[ManifestTypeIndex].(*Index)
	index.Components[id] = compInfo
	if isDefault {
		index.DefaultComponents = append(index.DefaultComponents, id)
	}
	index.Version += 1 // bump index version

	// update snapshot
	snapshot := manifests[ManifestTypeSnapshot].(*Snapshot).SetVersions(manifests)
	snapshot.Expires = currTime.Add(ManifestsConfig[ManifestTypeSnapshot].Expire).Format(time.RFC3339)

	// update timestamp
	timestamp, err := NewTimestamp(currTime).SetSnapshot(snapshot)
	if err != nil {
		return err
	}
	timestamp.Version = manifests[ManifestTypeTimestamp].(*Timestamp).Version + 1
	manifests[ManifestTypeTimestamp] = timestamp

	return BatchSaveManifests(repo, manifests, privKey)
}

// NewRoot creates a Root object
func NewRoot(initTime time.Time) *Root {
	return &Root{
		SignedBase: SignedBase{
			Ty:          ManifestTypeRoot,
			SpecVersion: CurrentSpecVersion,
			Expires:     initTime.Add(ManifestsConfig[ManifestTypeRoot].Expire).Format(time.RFC3339),
			Version:     1, // initial repo starts with version 1
		},
		Roles: make(map[string]*Role),
	}
}

// NewIndex creates a Index object
func NewIndex(initTime time.Time) *Index {
	return &Index{
		SignedBase: SignedBase{
			Ty:          ManifestTypeIndex,
			SpecVersion: CurrentSpecVersion,
			Expires:     initTime.Add(ManifestsConfig[ManifestTypeIndex].Expire).Format(time.RFC3339),
			Version:     1,
		},
		Owners:            make(map[string]Owner),
		Components:        make(map[string]ComponentItem),
		DefaultComponents: make([]string, 0),
	}
}

// NewSnapshot creates a Snapshot object.
func NewSnapshot(initTime time.Time) *Snapshot {
	return &Snapshot{
		SignedBase: SignedBase{
			Ty:          ManifestTypeSnapshot,
			SpecVersion: CurrentSpecVersion,
			Expires:     initTime.Add(ManifestsConfig[ManifestTypeSnapshot].Expire).Format(time.RFC3339),
			Version:     0, // not versioned
		},
	}
}

// NewTimestamp creates a Timestamp object
func NewTimestamp(initTime time.Time) *Timestamp {
	return &Timestamp{
		SignedBase: SignedBase{
			Ty:          ManifestTypeTimestamp,
			SpecVersion: CurrentSpecVersion,
			Expires:     initTime.Add(ManifestsConfig[ManifestTypeTimestamp].Expire).Format(time.RFC3339),
			Version:     1,
		},
	}
}

// NewComponent creates a Component object
func NewComponent(id, name, desc string, initTime time.Time) *Component {
	return &Component{
		SignedBase: SignedBase{
			Ty:          ManifestTypeComponent,
			SpecVersion: CurrentSpecVersion,
			Expires:     initTime.Add(ManifestsConfig[ManifestTypeComponent].Expire).Format(time.RFC3339),
			Version:     1,
		},
		ID:          id,
		Name:        name,
		Description: desc,
		Platforms:   make(map[string]map[string]VersionItem),
	}
}

// NewKeyInfo creates a KeyInfo object
func NewKeyInfo(priv crypto.PrivKey) (*KeyInfo, error) {
	pubBytes, err := priv.Public().Serialize()
	if err != nil {
		return nil, err
	}
	return &KeyInfo{
		Algorithms: []string{"sha256"},
		Type:       "rsa",
		Value: map[string]string{
			"public": string(pubBytes),
		},
		Scheme: "rsassa-pss-sha256",
	}, nil
}

// ID returns the SH256 hash of the key
func (k *KeyInfo) ID() (string, error) {
	info, err := cjson.Marshal(k)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(info)
	return hex.EncodeToString(hash[:]), nil
}

// SetVersions sets file versions to the snapshot
func (manifest *Snapshot) SetVersions(manifestList map[string]ValidManifest) *Snapshot {
	if manifest.Meta == nil {
		manifest.Meta = make(map[string]FileVersion)
	}
	for _, m := range manifestList {
		manifest.Meta[m.Filename()] = FileVersion{
			Version: m.Base().Version,
			// TODO length
		}
	}
	return manifest
}

// SetSnapshot hashes a snapshot manifest and update the timestamp manifest
func (manifest *Timestamp) SetSnapshot(s *Snapshot) (*Timestamp, error) {
	bytes, err := json.Marshal(s)
	if err != nil {
		return manifest, err
	}

	// TODO: hash the manifest

	if manifest.Meta == nil {
		manifest.Meta = make(map[string]FileHash)
	}
	manifest.Meta[s.Base().Filename()] = FileHash{
		Hashes: map[string]string{"sha256": "TODO"},
		Length: uint(len(bytes)),
	}

	return manifest, nil
}

// SetRole populates role list in the root manifest
func (manifest *Root) SetRole(m ValidManifest) {
	if manifest.Roles == nil {
		manifest.Roles = make(map[string]*Role)
	}

	manifest.Roles[m.Base().Ty] = &Role{
		URL:       m.Filename(),
		Threshold: ManifestsConfig[m.Base().Ty].Threshold,
		Keys:      make(map[string]*KeyInfo),
	}
}

// FreshKeyInfo generates a new key pair and wraps it in a KeyInfo. The returned string is the key id.
func FreshKeyInfo() (*KeyInfo, string, crypto.PrivKey, error) {
	pub, priv, err := crypto.RSAPair()
	if err != nil {
		return nil, "", nil, err
	}
	pubBytes, err := pub.Serialize()
	if err != nil {
		return nil, "", nil, err
	}
	info := KeyInfo{
		Algorithms: []string{"sha256"},
		Type:       "rsa",
		Value:      map[string]string{"public": string(pubBytes)},
		Scheme:     "rsassa-pss-sha256",
	}
	serInfo, err := cjson.Marshal(&info)
	if err != nil {
		return nil, "", nil, err
	}
	hash := sha256.Sum256(serInfo)

	return &info, fmt.Sprintf("%x", hash), priv, nil
}

// ReadManifestDir reads manifests from a dir
func ReadManifestDir(dir string) (map[string]ValidManifest, error) {
	manifests := make(map[string]ValidManifest)
	for ty, val := range ManifestsConfig {
		if val.Filename == "" {
			continue
		}
		reader, err := os.Open(filepath.Join(dir, val.Filename))
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		var role ValidManifest
		m, err := ReadManifest(reader, role, crypto.NewKeyStore())
		if err != nil {
			return nil, err
		}
		manifests[ty] = m.Signed
	}
	return manifests, nil
}

// SignAndWrite creates a manifest and writes it to out.
func SignAndWrite(out io.Writer, role ValidManifest, privKey crypto.PrivKey) error {
	payload, err := cjson.Marshal(role)
	if err != nil {
		return err
	}

	sign, err := privKey.Signature(payload)
	if err != nil {
		return errors.Trace(err)
	}

	keyInfo, err := NewKeyInfo(privKey)
	if err != nil {
		return err
	}
	keyID, err := keyInfo.ID()
	if err != nil {
		return err
	}

	manifest := Manifest{
		Signatures: []signature{{
			KeyID: keyID,
			Sig:   string(sign),
		}},
		Signed: role,
	}

	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "\t")
	return encoder.Encode(manifest)
}

// BatchSaveManifests write a series of manifests to disk
func BatchSaveManifests(dst string, manifestList map[string]ValidManifest, privKey crypto.PrivKey) error {
	for _, m := range manifestList {
		writer, err := os.OpenFile(filepath.Join(dst, m.Filename()), os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
		if err != nil {
			return err
		}
		defer writer.Close()
		// TODO: support multiples keys

		if err = SignAndWrite(writer, m, privKey); err != nil {
			return err
		}
	}
	return nil
}
