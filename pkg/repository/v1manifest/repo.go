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
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"time"

	cjson "github.com/gibson042/canonicaljson-go"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/crypto"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/utils"
)

// ErrorInsufficientKeys indicates that the key number is less than threshold
var ErrorInsufficientKeys = stderrors.New("not enough keys supplied")

// Init creates and initializes an empty reposityro
func Init(dst, keyDir string, initTime time.Time) (err error) {
	// initial manifests
	manifests := make(map[string]ValidManifest)
	signedManifests := make(map[string]*Manifest)

	// TODO: bootstrap a server instead of generating key
	keys := map[string][]*KeyInfo{}
	for _, ty := range []string{ManifestTypeRoot, ManifestTypeIndex, ManifestTypeSnapshot, ManifestTypeTimestamp} {
		if err := GenAndSaveKeys(keys, ty, int(ManifestsConfig[ty].Threshold), keyDir); err != nil {
			return err
		}
	}

	// init the root manifest
	manifests[ManifestTypeRoot] = NewRoot(initTime)

	// init index
	manifests[ManifestTypeIndex] = NewIndex(initTime)

	// init snapshot
	manifests[ManifestTypeSnapshot] = NewSnapshot(initTime)

	// init timestamp
	manifests[ManifestTypeTimestamp] = NewTimestamp(initTime)

	// root and snapshot has meta of each other inside themselves, but it's ok here
	// as we are still during the init process, not version bump needed
	for ty, val := range ManifestsConfig {
		if val.Filename == "" {
			// skip unsupported ManifestsConfig such as component
			continue
		}
		if m, ok := manifests[ty]; ok {
			if err := manifests[ManifestTypeRoot].(*Root).SetRole(m, keys[ty]...); err != nil {
				return err
			}
			continue
		}
		// FIXME: log a warning about manifest not found instead of returning error
		return fmt.Errorf("manifest '%s' not initialized porperly", ty)
	}

	if signedManifests[ManifestTypeRoot], err = SignManifest(manifests[ManifestTypeRoot], keys[ManifestTypeRoot]...); err != nil {
		return err
	}

	if signedManifests[ManifestTypeIndex], err = SignManifest(manifests[ManifestTypeIndex], keys[ManifestTypeIndex]...); err != nil {
		return err
	}

	if _, err = manifests[ManifestTypeSnapshot].(*Snapshot).SetVersions(signedManifests); err != nil {
		return err
	}

	if signedManifests[ManifestTypeSnapshot], err = SignManifest(manifests[ManifestTypeSnapshot], keys[ManifestTypeSnapshot]...); err != nil {
		return err
	}

	if _, err = manifests[ManifestTypeTimestamp].(*Timestamp).SetSnapshot(signedManifests[ManifestTypeSnapshot]); err != nil {
		return err
	}

	if signedManifests[ManifestTypeTimestamp], err = SignManifest(manifests[ManifestTypeTimestamp], keys[ManifestTypeTimestamp]...); err != nil {
		return err
	}

	return BatchSaveManifests(dst, signedManifests)
}

// SaveKeyInfo saves a KeyInfo object to a JSON file
func SaveKeyInfo(key *KeyInfo, ty, dir string) (string, error) {
	id, err := key.ID()
	if err != nil {
		return "", err
	}

	if dir == "" {
		dir, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	if utils.IsNotExist(dir) {
		if err := utils.MkdirAll(dir, 0755); err != nil {
			return "", errors.Annotate(err, "create key directory")
		}
	}

	pubPath := path.Join(dir, fmt.Sprintf("%s-%s.json", id[:ShortKeyIDLength], ty))
	f, err := os.Create(pubPath)
	if err != nil {
		return pubPath, err
	}
	defer f.Close()

	if _, found := key.Value["private"]; found {
		err = f.Chmod(0600)
		if err != nil {
			return pubPath, err
		}
	}

	return pubPath, json.NewEncoder(f).Encode(key)
}

// GenAndSaveKeys generate private keys to keys param and save key file to dir
func GenAndSaveKeys(keys map[string][]*KeyInfo, ty string, num int, dir string) error {
	for i := 0; i < num; i++ {
		k, err := GenKeyInfo()
		if err != nil {
			return err
		}
		keys[ty] = append(keys[ty], k)

		if _, err := SaveKeyInfo(k, ty, dir); err != nil {
			return err
		}
	}
	return nil
}

// SignManifestData add signatures to a manifest data
func SignManifestData(data []byte, ki *KeyInfo) ([]byte, error) {
	m := RawManifest{}

	if err := json.Unmarshal(data, &m); err != nil {
		return nil, errors.Annotate(err, "unmarshal manifest")
	}

	var signed any
	if err := json.Unmarshal(m.Signed, &signed); err != nil {
		return nil, errors.Annotate(err, "unmarshal manifest.signed")
	}

	payload, err := cjson.Marshal(signed)
	if err != nil {
		return nil, errors.Annotate(err, "marshal manifest.signed")
	}

	id, err := ki.ID()
	if err != nil {
		return nil, err
	}

	sig, err := ki.Signature(payload)
	if err != nil {
		return nil, err
	}

	for _, s := range m.Signatures {
		if s.KeyID == id {
			s.Sig = sig
			return nil, errors.New("this manifest file has already been signed by specified key")
		}
	}

	m.Signatures = append(m.Signatures, Signature{
		KeyID: id,
		Sig:   sig,
	})

	content, err := cjson.Marshal(m)
	if err != nil {
		return nil, errors.Annotate(err, "marshal signed manifest")
	}

	return content, nil
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
		Meta: make(map[string]FileVersion),
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
func NewComponent(id, desc string, initTime time.Time) *Component {
	return &Component{
		SignedBase: SignedBase{
			Ty:          ManifestTypeComponent,
			SpecVersion: CurrentSpecVersion,
			Expires:     initTime.Add(ManifestsConfig[ManifestTypeComponent].Expire).Format(time.RFC3339),
			Version:     1,
		},
		ID:          id,
		Description: desc,
		Platforms:   make(map[string]map[string]VersionItem),
	}
}

// SetVersions sets file versions to the snapshot
func (manifest *Snapshot) SetVersions(manifestList map[string]*Manifest) (*Snapshot, error) {
	if manifest.Meta == nil {
		manifest.Meta = make(map[string]FileVersion)
	}
	for _, m := range manifestList {
		bytes, err := cjson.Marshal(m)
		if err != nil {
			return nil, err
		}
		manifest.Meta["/"+m.Signed.Filename()] = FileVersion{
			Version: m.Signed.Base().Version,
			Length:  uint(len(bytes)),
		}
	}
	return manifest, nil
}

// SetSnapshot hashes a snapshot manifest and update the timestamp manifest
func (manifest *Timestamp) SetSnapshot(s *Manifest) (*Timestamp, error) {
	bytes, err := cjson.Marshal(s)
	if err != nil {
		return manifest, err
	}

	hash256 := sha256.Sum256(bytes)
	hash512 := sha512.Sum512(bytes)

	if manifest.Meta == nil {
		manifest.Meta = make(map[string]FileHash)
	}
	manifest.Meta[fmt.Sprintf("/%s", s.Signed.Base().Filename())] = FileHash{
		Hashes: map[string]string{
			SHA256: hex.EncodeToString(hash256[:]),
			SHA512: hex.EncodeToString(hash512[:]),
		},
		Length: uint(len(bytes)),
	}

	return manifest, nil
}

// SetRole populates role list in the root manifest
func (manifest *Root) SetRole(m ValidManifest, keys ...*KeyInfo) error {
	if manifest.Roles == nil {
		manifest.Roles = make(map[string]*Role)
	}

	manifest.Roles[m.Base().Ty] = &Role{
		URL:       fmt.Sprintf("/%s", m.Filename()),
		Threshold: ManifestsConfig[m.Base().Ty].Threshold,
		Keys:      make(map[string]*KeyInfo),
	}

	if uint(len(keys)) < manifest.Roles[m.Base().Ty].Threshold {
		return ErrorInsufficientKeys
	}

	for _, k := range keys {
		id, err := k.ID()
		if err != nil {
			return err
		}
		pub, err := k.Public()
		if err != nil {
			return err
		}
		manifest.Roles[m.Base().Ty].Keys[id] = pub
	}

	return nil
}

// AddKey adds a public key info to a role of Root
func (manifest *Root) AddKey(roleName string, key *KeyInfo) error {
	newID, err := key.ID()
	if err != nil {
		return err
	}
	role, found := manifest.Roles[roleName]
	if !found {
		return errors.Errorf("role '%s' not found in root manifest", roleName)
	}
	for _, k := range role.Keys {
		id, err := k.ID()
		if err != nil {
			return err
		}
		if newID == id {
			return nil // skip exist
		}
	}
	role.Keys[newID] = key
	return nil
}

// FreshKeyInfo generates a new key pair and wraps it in a KeyInfo. The returned string is the key id.
func FreshKeyInfo() (*KeyInfo, string, crypto.PrivKey, error) {
	priv, err := crypto.NewKeyPair(crypto.KeyTypeRSA, crypto.KeySchemeRSASSAPSSSHA256)
	if err != nil {
		return nil, "", nil, err
	}
	pubBytes, err := priv.Public().Serialize()
	if err != nil {
		return nil, "", nil, err
	}
	info := KeyInfo{
		Type:   "rsa",
		Value:  map[string]string{"public": string(pubBytes)},
		Scheme: "rsassa-pss-sha256",
	}
	serInfo, err := cjson.Marshal(&info)
	if err != nil {
		return nil, "", nil, err
	}
	hash := sha256.Sum256(serInfo)

	return &info, fmt.Sprintf("%x", hash), priv, nil
}

// ReadManifestDir reads manifests from a dir
func ReadManifestDir(dir string, roles ...string) (map[string]ValidManifest, error) {
	manifests := make(map[string]ValidManifest)
	roleSet := set.NewStringSet(roles...)
	for ty, val := range ManifestsConfig {
		if len(roles) > 0 && !roleSet.Exist(ty) {
			continue // skip unlisted
		}
		if val.Filename == "" {
			continue
		}
		reader, err := os.Open(filepath.Join(dir, val.Filename))
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		var role ValidManifest
		m, err := ReadManifest(reader, role, nil)
		if err != nil {
			return nil, err
		}
		manifests[ty] = m.Signed
	}
	return manifests, nil
}

// SignManifest signs a manifest with given private key
func SignManifest(role ValidManifest, keys ...*KeyInfo) (*Manifest, error) {
	payload, err := cjson.Marshal(role)
	if err != nil {
		return nil, err
	}

	signs := []Signature{}
	for _, k := range keys {
		id, err := k.ID()
		if err != nil {
			return nil, errors.Trace(err)
		}
		sign, err := k.Signature(payload)
		if err != nil {
			return nil, errors.Trace(err)
		}
		signs = append(signs, Signature{
			KeyID: id,
			Sig:   sign,
		})
	}

	return &Manifest{
		Signatures: signs,
		Signed:     role,
	}, nil
}

// WriteManifestFile writes a Manifest object to file in JSON format
func WriteManifestFile(fname string, m *Manifest) error {
	writer, err := os.OpenFile(fname, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer writer.Close()
	return WriteManifest(writer, m)
}

// WriteManifest writes a Manifest object to writer in JSON format
func WriteManifest(out io.Writer, m *Manifest) error {
	bytes, err := cjson.Marshal(m)
	if err != nil {
		return errors.Trace(err)
	}
	_, err = out.Write(bytes)
	return err
}

// SignAndWrite creates a manifest and writes it to out.
func SignAndWrite(out io.Writer, role ValidManifest, keys ...*KeyInfo) error {
	manifest, err := SignManifest(role, keys...)
	if err != nil {
		return errors.Trace(err)
	}

	return WriteManifest(out, manifest)
}

// BatchSaveManifests write a series of manifests to disk
// Manifest in the manifestList map should already be signed, they are not checked
// for signature again.
func BatchSaveManifests(dst string, manifestList map[string]*Manifest) error {
	for ty, m := range manifestList {
		filename := m.Signed.Filename()
		if ty == ManifestTypeIndex {
			filename = fmt.Sprintf("%d.%s", m.Signed.Base().Version, m.Signed.Filename())
		}
		writer, err := os.OpenFile(filepath.Join(dst, filename), os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
		if err != nil {
			return err
		}
		defer writer.Close()

		if err = WriteManifest(writer, m); err != nil {
			return err
		}
	}
	return nil
}
