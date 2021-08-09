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
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	cjson "github.com/gibson042/canonicaljson-go"
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

	// AnyPlatform is the ID for platform independent components
	AnyPlatform = "any/any"

	// Acceptable values for hash kinds.
	SHA256 = "sha256"
	SHA512 = "sha512"
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
		Expire:    time.Hour * 24 * 365 * 5, // 5y
		Threshold: 1,
	},
	ManifestTypeSnapshot: {
		Filename:  ManifestFilenameSnapshot,
		Versioned: false,
		Expire:    time.Hour * 24 * 30, // 1mon, should be changed to 1d
		Threshold: 1,
	},
	ManifestTypeTimestamp: {
		Filename:  ManifestFilenameTimestamp,
		Versioned: false,
		Expire:    time.Hour * 24 * 30, // 1mon, should be changed to 1d
		Threshold: 1,
	},
}

var knownVersions = map[string]bool{
	"0.1.0": true,
}

// AddSignature adds one or more signatures to the manifest
func (manifest *Manifest) AddSignature(sigs []Signature) {
	hasSig := func(lst []Signature, s Signature) bool {
		for _, sig := range lst {
			if sig.KeyID == s.KeyID {
				return true
			}
		}
		return false
	}
	for _, sig := range sigs {
		if hasSig(manifest.Signatures, sig) {
			continue
		}
		manifest.Signatures = append(manifest.Signatures, sig)
	}
}

// ExpirationError the a manifest has expired.
type ExpirationError struct {
	fname string
	date  string
}

func (s *ExpirationError) Error() string {
	return fmt.Sprintf("manifest %s has expired at: %s", s.fname, s.date)
}

func newExpirationError(fname, date string) *ExpirationError {
	return &ExpirationError{
		fname: fname,
		date:  date,
	}
}

// IsExpirationError checks if the err is an ExpirationError.
func IsExpirationError(err error) bool {
	_, ok := err.(*ExpirationError)
	return ok
}

// ComponentManifestFilename returns the expected filename for the component manifest identified by id.
func ComponentManifestFilename(id string) string {
	return fmt.Sprintf("%s.json", id)
}

// RootManifestFilename returns the expected filename for the root manifest with the given version.
func RootManifestFilename(version uint) string {
	return fmt.Sprintf("%v.root.json", version)
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

// CheckExpiry return not nil if it's expired.
func CheckExpiry(fname, expires string) error {
	expiresTime, err := time.Parse(time.RFC3339, expires)
	if err != nil {
		return errors.AddStack(err)
	}

	if expiresTime.Before(time.Now()) {
		return newExpirationError(fname, expires)
	}

	return nil
}

// ExpiresAfter checks that manifest 1 expires after manifest 2 (or are equal) and returns an error otherwise.
func ExpiresAfter(m1, m2 ValidManifest) error {
	time1, err := time.Parse(time.RFC3339, m1.Base().Expires)
	if err != nil {
		return errors.AddStack(err)
	}
	time2, err := time.Parse(time.RFC3339, m2.Base().Expires)
	if err != nil {
		return errors.AddStack(err)
	}

	if time1.Before(time2) {
		return fmt.Errorf("manifests have mis-ordered expiry times, expected %s >= %s", time1, time2)
	}

	return nil
}

// isValid checks if s is valid manifest metadata.
func (s *SignedBase) isValid(filename string) error {
	if _, ok := ManifestsConfig[s.Ty]; !ok {
		return fmt.Errorf("unknown manifest type: `%s`", s.Ty)
	}

	if !knownVersions[s.SpecVersion] {
		return fmt.Errorf("unknown manifest version: `%s`, you might need to update TiUP", s.SpecVersion)
	}

	// When updating root, we only check the newest version is not expire.
	// This checking should be done by the update root flow.
	if s.Ty != ManifestTypeRoot {
		if err := CheckExpiry(filename, s.Expires); err != nil {
			return err
		}
	}

	return nil
}

func (manifest *Root) isValid() error {
	types := []string{ManifestTypeRoot, ManifestTypeIndex, ManifestTypeSnapshot, ManifestTypeTimestamp}
	for _, ty := range types {
		role, ok := manifest.Roles[ty]
		if !ok {
			return fmt.Errorf("root manifest is missing %s role", ty)
		}
		if uint(len(role.Keys)) < role.Threshold {
			return fmt.Errorf("%s role in root manifest does not have enough keys; expected: %v, found: %v", ty, role.Threshold, len(role.Keys))
		}

		// Check all keys have valid id and known types.
		for id, k := range role.Keys {
			serInfo, err := cjson.Marshal(k)
			if err != nil {
				return err
			}
			hash := fmt.Sprintf("%x", sha256.Sum256(serInfo))
			if hash != id {
				return fmt.Errorf("id does not match key. Expected: %s, found %s", hash, id)
			}

			if k.Type != "rsa" {
				return fmt.Errorf("unsupported key type %s in key %s", k.Type, id)
			}
			if k.Scheme != "rsassa-pss-sha256" {
				return fmt.Errorf("unsupported scheme %s in key %s", k.Scheme, id)
			}
		}
	}

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
	// Nothing to do.
	return nil
}

// VersionItem returns VersionItem by platform and version
func (manifest *Component) VersionItem(plat, ver string, includeYanked bool) *VersionItem {
	var v VersionItem
	var ok bool

	if includeYanked {
		v, ok = manifest.VersionListWithYanked(plat)[ver]
	} else {
		v, ok = manifest.VersionList(plat)[ver]
	}
	if !ok || v.Entry == "" {
		return nil
	}
	return &v
}

func (manifest *Snapshot) isValid() error {
	// Nothing to do.
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

// VersionedURL looks up url in the snapshot and returns a modified url with the version prefix, and that file's length.
func (manifest *Snapshot) VersionedURL(url string) (string, *FileVersion, error) {
	entry, ok := manifest.Meta[url]
	if !ok {
		return "", nil, fmt.Errorf("no entry in snapshot manifest for %s", url)
	}
	lastSlash := strings.LastIndex(url, "/")
	if lastSlash < 0 {
		return fmt.Sprintf("%v.%s", entry.Version, url), &entry, nil
	}

	return fmt.Sprintf("%s/%v.%s", url[:lastSlash], entry.Version, url[lastSlash+1:]), &entry, nil
}

// ReadComponentManifest reads a component manifest from input and validates it.
func ReadComponentManifest(input io.Reader, com *Component, item *ComponentItem, keys *KeyStore) (*Manifest, error) {
	decoder := json.NewDecoder(input)
	// For prevent the signatures verify failed from specification changing
	// we use JSON raw message decode the signed part first.
	rawM := RawManifest{}
	if err := decoder.Decode(&rawM); err != nil {
		return nil, errors.Trace(err)
	}
	if err := json.Unmarshal(rawM.Signed, com); err != nil {
		return nil, errors.Trace(err)
	}

	err := keys.verifySignature(rawM.Signed, item.Owner, rawM.Signatures, com.Filename())
	if err != nil {
		return nil, errors.Trace(err)
	}

	m := &Manifest{
		Signatures: rawM.Signatures,
		Signed:     com,
	}

	err = m.Signed.Base().isValid(m.Signed.Filename())
	if err != nil {
		return nil, errors.Trace(err)
	}

	return m, m.Signed.isValid()
}

// ReadNoVerify will read role from input and will not do any validation or verification. It is very dangerous to use
// this function and it should only be used to read trusted data from local storage.
func ReadNoVerify(input io.Reader, role ValidManifest) (*Manifest, error) {
	decoder := json.NewDecoder(input)
	var m Manifest
	m.Signed = role
	return &m, decoder.Decode(&m)
}

// ReadManifest reads a manifest from input and validates it, the result is stored in role, which must be a pointer type.
func ReadManifest(input io.Reader, role ValidManifest, keys *KeyStore) (*Manifest, error) {
	if role.Base().Ty == ManifestTypeComponent {
		return nil, errors.New("trying to read component manifest as non-component manifest")
	}

	decoder := json.NewDecoder(input)
	// To prevent signatures verification failure from specification changing
	// we use JSON raw message decode the signed part first.
	rawM := RawManifest{}
	if err := decoder.Decode(&rawM); err != nil {
		return nil, errors.Trace(err)
	}
	if err := cjson.Unmarshal(rawM.Signed, role); err != nil {
		return nil, errors.Trace(err)
	}

	err := keys.verifySignature(rawM.Signed, role.Base().Ty, rawM.Signatures, role.Base().Filename())
	if err != nil {
		return nil, errors.Trace(err)
	}

	m := &Manifest{
		Signatures: rawM.Signatures,
		Signed:     role,
	}

	if role.Base().Ty == ManifestTypeRoot {
		newRoot := role.(*Root)
		threshold := newRoot.Roles[ManifestTypeRoot].Threshold

		err = keys.transitionRoot(rawM.Signed, threshold, newRoot.Expires, m.Signatures, newRoot.Roles[ManifestTypeRoot].Keys)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}

	err = m.Signed.Base().isValid(m.Signed.Filename())
	if err != nil {
		return nil, errors.Trace(err)
	}

	return m, m.Signed.isValid()
}

// RenewManifest resets and extends the expire time of manifest
func RenewManifest(m ValidManifest, startTime time.Time, extend ...time.Duration) {
	// manifest with 0 version means it's unversioned
	if m.Base().Version > 0 {
		m.Base().Version++
	}

	// only update expire field when it's older than target expire time
	duration := ManifestsConfig[m.Base().Ty].Expire
	if len(extend) > 0 {
		duration = extend[0]
	}
	targetExpire := startTime.Add(duration)
	currentExpire, err := time.Parse(time.RFC3339, m.Base().Expires)
	if err != nil {
		m.Base().Expires = targetExpire.Format(time.RFC3339)
		return
	}

	if currentExpire.Before(targetExpire) {
		m.Base().Expires = targetExpire.Format(time.RFC3339)
	}
}

// loadKeys stores all keys declared in manifest into ks.
func loadKeys(manifest ValidManifest, ks *KeyStore) error {
	switch manifest.Base().Ty {
	case ManifestTypeRoot:
		root := manifest.(*Root)
		for name, role := range root.Roles {
			if err := ks.AddKeys(name, role.Threshold, manifest.Base().Expires, role.Keys); err != nil {
				return errors.Trace(err)
			}
		}
	case ManifestTypeIndex:
		index := manifest.(*Index)
		for name, owner := range index.Owners {
			if err := ks.AddKeys(name, uint(owner.Threshold), manifest.Base().Expires, owner.Keys); err != nil {
				return errors.Trace(err)
			}
		}
	}
	return nil
}
