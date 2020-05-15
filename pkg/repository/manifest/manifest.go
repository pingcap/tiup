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

package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	cjson "github.com/gibson042/canonicaljson-go"
	"github.com/pingcap-incubator/tiup/pkg/repository/crypto"
)

// Names of manifest types
const (
	ManifestTypeRoot      = "root"
	ManifestTypeIndex     = "index"
	ManifestTypeSnapshot  = "snapshot"
	ManifestTypeTimestamp = "timestamp"
	//ManifestTypeComponent = "component"

	// SpecVersion of current, maybe we could expand it later
	CurrentSpecVersion = "0.1.0"
)

// ty is type information about a manifest
type ty struct {
	filename  string
	versioned bool
	expire    time.Duration
	threshold uint
}

// meta configs for different manifest types
var types = map[string]ty{
	"root": {
		filename:  ManifestTypeRoot + ".json",
		versioned: true,
		expire:    time.Hour * 24 * 365, // 1y
		threshold: 3,
	},
	"index": {
		filename:  ManifestTypeIndex + ".json",
		versioned: true,
		expire:    time.Hour * 24 * 365, // 1y
		threshold: 1,
	},
	"component": {
		filename:  "",
		versioned: true,
		expire:    time.Hour * 24 * 365, // 1y
		threshold: 1,
	},
	"snapshot": {
		filename:  ManifestTypeSnapshot + ".json",
		versioned: false,
		expire:    time.Hour * 24, // 1d
		threshold: 1,
	},
	"timestamp": {
		filename:  ManifestTypeTimestamp + ".json",
		versioned: false,
		expire:    time.Hour * 24, // 1d
		threshold: 1,
	},
}

var knownVersions = map[string]struct{}{"0.1.0": {}}

// verifySignature ensures that each signature in manifest::signatures is a valid signature of manifest::signed.
func (manifest *Manifest) verifySignature(keys crypto.KeyStore) error {
	if keys == nil {
		return nil
	}

	payload, err := cjson.Marshal(manifest.Signed)
	if err != nil {
		return nil
	}

	for _, sig := range manifest.Signatures {
		key := keys.Get(sig.KeyID)
		if key == nil {
			return fmt.Errorf("signature key %s not found", sig.KeyID)
		}
		if err := key.Verify(payload, sig.Sig); err != nil {
			return err
		}
	}

	return nil
}

// SignatureError the signature of a file is incorrect.
type SignatureError struct{}

func (err *SignatureError) Error() string {
	// TODO include the filename
	return "invalid signature for file"
}

// Filename returns the unversioned name that the manifest should be saved as based on the type in s.
func (s *SignedBase) Filename() string {
	return types[s.Ty].filename
}

// Versioned indicates whether versioned versions of a manifest are saved, e.g., 42.foo.json.
func (s *SignedBase) Versioned() bool {
	return types[s.Ty].versioned
}

func (s *SignedBase) expiryTime() (time.Time, error) {
	return time.Parse(time.RFC3339, s.Expires)
}

// isValid checks if s is valid manifest metadata.
func (s *SignedBase) isValid() error {
	if _, ok := types[s.Ty]; !ok {
		return fmt.Errorf("unknown manifest type: `%s`", s.Ty)
	}

	if _, ok := knownVersions[s.SpecVersion]; !ok {
		return fmt.Errorf("unknown manifest version: `%s`", s.SpecVersion)
	}

	expires, err := s.expiryTime()
	if err != nil {
		return err
	}

	if expires.Before(time.Now()) {
		return fmt.Errorf("manifest has expired at: %s", s.Expires)
	}

	return nil
}

func (manifest *Root) isValid() error {
	return nil
}

func (manifest *Index) isValid() error {
	return nil
}

func (manifest *Component) isValid() error {
	return nil
}

func (manifest *Snapshot) isValid() error {
	return nil
}

func (manifest *Timestamp) isValid() error {
	snapshot, ok := manifest.Meta["snapshot.json"]
	if !ok {
		return errors.New("timestamp manifest is missing entry for snapshot.json")
	}
	if len(manifest.Meta) > 1 {
		return errors.New("timestamp manifest has too many entries in `meta`")
	}
	if len(snapshot.Hashes) == 0 {
		return errors.New("timestamp manifest missing hash for snapshot.json")
	}
	return nil
}

// SnapshotHash returns the hashes of the snapshot manifest as specified in the timestamp manifest.
func (manifest *Timestamp) SnapshotHash() FileHash {
	return manifest.Meta[types[ManifestTypeSnapshot].filename]
}

// ReadManifest reads a manifest from input and validates it, the result is stored in role, which must be a pointer type.
func ReadManifest(input io.Reader, role ValidManifest, keys crypto.KeyStore) (*Manifest, error) {
	decoder := json.NewDecoder(input)
	var m Manifest
	m.Signed = role
	err := decoder.Decode(&m)
	if err != nil {
		return nil, err
	}

	if len(m.Signatures) == 0 {
		return nil, errors.New("no signatures supplied in manifest")
	}

	err = m.verifySignature(keys)
	if err != nil {
		return nil, err
	}

	err = m.Signed.Base().isValid()
	if err != nil {
		return nil, err
	}

	return &m, m.Signed.isValid()
}

func readTimestampManifest(input io.Reader, keys crypto.KeyStore) (*Timestamp, error) {
	var ts Timestamp
	_, err := ReadManifest(input, &ts, keys)
	if err != nil {
		return nil, err
	}

	return &ts, nil
}

// batchSaveManifests write a series of manifests to disk
func batchSaveManifests(dst string, manifestList map[string]ValidManifest) error {
	for _, m := range manifestList {
		writer, err := os.OpenFile(filepath.Join(dst, m.Filename()), os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
		if err != nil {
			return err
		}
		defer writer.Close()
		if err = SignAndWrite(writer, m); err != nil {
			return err
		}
	}
	return nil
}
