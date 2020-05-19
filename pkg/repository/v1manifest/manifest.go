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
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	cjson "github.com/gibson042/canonicaljson-go"
	"github.com/pingcap-incubator/tiup/pkg/repository/crypto"
	"github.com/pingcap/errors"
)

// Names of manifest ManifestsConfig
const (
	ManifestTypeRoot      = "root"
	ManifestTypeIndex     = "index"
	ManifestTypeSnapshot  = "snapshot"
	ManifestTypeTimestamp = "timestamp"
	ManifestTypeComponent = "component"
	// Manifest URLs in a repository.
	ManifestURLRoot      = "/root.json"
	ManifestURLIndex     = "/index.json"
	ManifestURLSnapshot  = "/snapshot.json"
	ManifestURLTimestamp = "/timestamp.json"
	// Manifest filenames when stored locally.
	ManifestFilenameRoot      = "root.json"
	ManifestFilenameIndex     = "index.json"
	ManifestFilenameSnapshot  = "snapshot.json"
	ManifestFilenameTimestamp = "timestamp.json"

	// SpecVersion of current, maybe we could expand it later
	CurrentSpecVersion = "0.1.0"
)

// ty is type information about a manifest
type ty struct {
	Filename  string
	Versioned bool
	Expire    time.Duration
	Threshold uint
}

// ManifestsConfig for different manifest ManifestsConfig
var ManifestsConfig = map[string]ty{
	ManifestTypeRoot: {
		Filename:  ManifestFilenameRoot,
		Versioned: true,
		Expire:    time.Hour * 24 * 365, // 1y
		Threshold: 3,
	},
	ManifestTypeIndex: {
		Filename:  ManifestFilenameIndex,
		Versioned: true,
		Expire:    time.Hour * 24 * 365, // 1y
		Threshold: 1,
	},
	ManifestTypeComponent: {
		Filename:  "",
		Versioned: true,
		Expire:    time.Hour * 24 * 365, // 1y
		Threshold: 1,
	},
	ManifestTypeSnapshot: {
		Filename:  ManifestFilenameSnapshot,
		Versioned: false,
		Expire:    time.Hour * 24, // 1d
		Threshold: 1,
	},
	ManifestTypeTimestamp: {
		Filename:  ManifestFilenameTimestamp,
		Versioned: false,
		Expire:    time.Hour * 24, // 1d
		Threshold: 1,
	},
}

var knownVersions = map[string]struct{}{"0.1.0": {}}

// VerifySignature ensures that threshold signature in manifest::signatures is a valid signature of manifest::signed.
func (manifest *Manifest) VerifySignature(threshold uint, keys crypto.KeyStore) error {
	if keys == nil {
		return nil
	}

	if threshold == 0 {
		return nil
	}

	if len(manifest.Signatures) < int(threshold) {
		return errors.Errorf("only %d signature but need %d", len(manifest.Signatures), threshold)
	}

	payload, err := cjson.Marshal(manifest.Signed)
	if err != nil {
		return nil
	}

	var validCount uint
	var errs []error
	for _, sig := range manifest.Signatures {
		// TODO check that KeyID belongs to a role which is authorised to sign this manifest
		key := keys.Get(sig.KeyID)
		if key == nil {
			// TODO use SignatureError
			err := fmt.Errorf("signature key %s not found", sig.KeyID)
			errs = append(errs, err)
			continue
		}
		err := key.Verify(payload, sig.Sig)
		if err != nil {
			errs = append(errs, err)
		} else {
			validCount++
		}
	}

	if validCount >= threshold {
		return nil
	}

	return errors.Errorf("only %d signature valid but need %d", validCount, threshold)
}

// SignatureError the signature of a file is incorrect.
type SignatureError struct{}

func (err *SignatureError) Error() string {
	// TODO include the filename
	return "invalid signature for file"
}

// ComponentFilename returns the expected filename for the component identified by id.
func ComponentFilename(id string) string {
	return fmt.Sprintf("%s.json", id)
}

// Filename returns the unversioned name that the manifest should be saved as based on the type in s.
func (s *SignedBase) Filename() string {
	fname := ManifestsConfig[s.Ty].Filename
	if fname == "" {
		panic("Unreachable")
	}
	return fname
}

// Versioned indicates whether versioned versions of a manifest are saved, e.g., 42.foo.json.
func (s *SignedBase) Versioned() bool {
	return ManifestsConfig[s.Ty].Versioned
}

// CheckExpire return not nil if it's expired.
func CheckExpire(expires string) error {
	expiresTime, err := time.Parse(time.RFC3339, expires)
	if err != nil {
		return errors.AddStack(err)
	}

	if expiresTime.Before(time.Now()) {
		return fmt.Errorf("manifest has expired at: %s", expires)
	}

	return nil
}

func (s *SignedBase) expiryTime() (time.Time, error) {
	return time.Parse(time.RFC3339, s.Expires)
}

// isValid checks if s is valid manifest metadata.
func (s *SignedBase) isValid() error {
	if _, ok := ManifestsConfig[s.Ty]; !ok {
		return fmt.Errorf("unknown manifest type: `%s`", s.Ty)
	}

	if _, ok := knownVersions[s.SpecVersion]; !ok {
		return fmt.Errorf("unknown manifest version: `%s`", s.SpecVersion)
	}

	if err := CheckExpire(s.Expires); err != nil {
		return errors.AddStack(err)
	}

	return nil
}

func (manifest *Root) isValid() error {
	// TODO
	return nil
}

func (manifest *Index) isValid() error {
	// Check every component's owner exists.
	for k, c := range manifest.Components {
		if _, ok := manifest.Owners[c.Owner]; !ok {
			return fmt.Errorf("component %s has unknown owner %s", k, c.Owner)
		}
	}

	// Check every default is in component.
	for _, d := range manifest.DefaultComponents {
		if _, ok := manifest.Components[d]; !ok {
			return fmt.Errorf("default component %s is unknown", d)
		}
	}

	return nil
}

func (manifest *Component) isValid() error {
	// TODO
	return nil
}

func (manifest *Snapshot) isValid() error {
	// TODO
	return nil
}

func (manifest *Timestamp) isValid() error {
	snapshot, ok := manifest.Meta[ManifestURLSnapshot]
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
	return manifest.Meta[ManifestURLSnapshot]
}

// VersionedURL looks up url in the snapshot and returns a modified url with the version prefix
func (manifest *Snapshot) VersionedURL(url string) (string, error) {
	entry, ok := manifest.Meta[url]
	if !ok {
		return "", fmt.Errorf("no entry in snapshot manifest for %s", url)
	}
	lastSlash := strings.LastIndex(url, "/")
	if lastSlash < 0 {
		return fmt.Sprintf("%v.%s", entry.Version, url), nil
	}

	return fmt.Sprintf("%s/%v.%s", url[:lastSlash], entry.Version, url[lastSlash+1:]), nil
}

func readTimestampManifest(input io.Reader, keys crypto.KeyStore) (*Timestamp, error) {
	var ts Timestamp
	_, err := ReadManifest(input, &ts, keys)
	if err != nil {
		return nil, err
	}

	return &ts, nil
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

	err = m.VerifySignature(uint(len(m.Signatures)), keys)
	if err != nil {
		return nil, err
	}

	err = m.Signed.Base().isValid()
	if err != nil {
		return nil, err
	}

	return &m, m.Signed.isValid()
}
