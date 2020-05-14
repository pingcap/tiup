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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

type signature struct {
	KeyID string `json:"keyid"`
	Sig   string `json:"sig"`
}

// SignedBase represents parts of a manifest's signed value which are shared by all manifests.
type SignedBase struct {
	Ty          string `json:"_type"`
	SpecVersion string `json:"spec_version"`
	Expires     string `json:"expires"`
	// 0 => no version specified
	Version uint `json:"version"`
}

// Manifest representation for ser/de.
type Manifest struct {
	// Signatures value
	Signatures []signature `json:"signatures"`
	// Signed value; any value here must have the SignedBase base.
	Signed ValidManifest `json:"signed"`
}

// ValidManifest is a manifest which includes SignedBase and can be validated.
type ValidManifest interface {
	isValid() error
	// Base returns this manifest's SignedBase which is values common to all manifests.
	Base() *SignedBase
	// Filename returns the unversioned name that the manifest should be saved as based on its Go type.
	Filename() string
	// GetRole returns the meta info of the manifest
	GetRole() *Role
}

// ty is type information about a manifest
type ty struct {
	filename  string
	versioned bool
	threshold uint
}

var types = map[string]ty{
	"root":      {"root.json", true, 3},
	"index":     {"index.json", true, 1},
	"component": {"", true, 1},
	"snapshot":  {"snapshot.json", false, 1},
	"timestamp": {"timestamp.json", false, 1},
}
var knownVersions = map[string]struct{}{"0.1.0": {}}

// Role is the metadata of a manifest
type Role struct {
	URL       string               `json:"url"`
	Keys      map[string]*KeyStore `json:"keys"`
	Threshold uint                 `json:"threshold"`
}

// Root manifest
type Root struct {
	SignedBase
	Roles map[string]*Role `json:"roles"`
}

// NewRoot creates a Root object
func NewRoot(initTime time.Time) *Root {
	root := &Root{
		SignedBase: SignedBase{
			Ty:          "root",
			SpecVersion: "TODO",
			Expires:     initTime.Add(time.Hour * 24 * 365).Format(time.RFC3339), // 1y
			Version:     1,                                                       // initial repo starts with version 1
		},
		Roles: make(map[string]*Role),
	}
	root.Roles[root.Filename()] = &Role{
		URL:       fmt.Sprintf("/%s", root.Filename()),
		Threshold: types["root"].threshold, // root manifest has higher threshold
		//Keys:      &KeyStore{},
	}
	return root
}

// Index manifest
type Index struct {
	SignedBase
	Owners            map[string]Owner     `json:"owners"`
	Components        map[string]Component `json:"components"`
	DefaultComponents []string             `json:"default_components"`
}

// NewIndex creates a Index object
func NewIndex(initTime time.Time) *Index {
	return &Index{
		SignedBase: SignedBase{
			Ty:          "index",
			SpecVersion: "TODO",
			Expires:     initTime.Add(time.Hour * 24 * 365).Format(time.RFC3339), // 1y
			Version:     1,
		},
		Owners:            make(map[string]Owner),
		Components:        make(map[string]Component),
		DefaultComponents: make([]string, 0),
	}
}

// Owner manifest (inline object, not dedicated files)
type Owner struct {
	Name string               `json:"name"`
	Keys map[string]*KeyStore `json:"keys"`
}

// Component manifest
type Component struct {
	SignedBase
	// TODO
}

// Snapshot manifest.
type Snapshot struct {
	SignedBase
	Meta map[string]FileVersion `json:"meta"`
}

// NewSnapshot creates a Snapshot object.
func NewSnapshot(initTime time.Time) *Snapshot {
	return &Snapshot{
		SignedBase: SignedBase{
			Ty:          "snapshot",
			SpecVersion: "TODO",
			Expires:     initTime.Add(time.Hour * 24).Format(time.RFC3339), // 1d
			Version:     0,                                                 // not versioned
		},
	}
}

// Timestamp manifest.
type Timestamp struct {
	SignedBase
	Meta map[string]FileHash `json:"meta"`
}

// NewTimestamp creates a Timestamp object
func NewTimestamp(initTime time.Time) *Timestamp {
	return &Timestamp{
		SignedBase: SignedBase{
			Ty:          "timestamp",
			SpecVersion: "TODO",
			Expires:     initTime.Add(time.Hour * 24).Format(time.RFC3339), // 1d
			Version:     1,
		},
	}
}

// FileHash is the hashes and length of a file.
type FileHash struct {
	Hashes map[string]string `json:"hashes"`
	Length uint              `json:"length"`
}

// FileVersion is just a version number.
type FileVersion struct {
	Version uint `json:"version"`
}

// verifySignature ensures that each signature in manifest::signatures is a valid signature of manifest::signed.
func (manifest *Manifest) verifySignature(keys *KeyStore) error {
	// TODO
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
	return manifest.Meta[types["snapshot"].filename]
}

// Base implements ValidManifest
func (manifest *Root) Base() *SignedBase {
	return &manifest.SignedBase
}

// Base implements ValidManifest
func (manifest *Index) Base() *SignedBase {
	return &manifest.SignedBase
}

// Base implements ValidManifest
func (manifest *Component) Base() *SignedBase {
	return &manifest.SignedBase
}

// Base implements ValidManifest
func (manifest *Snapshot) Base() *SignedBase {
	return &manifest.SignedBase
}

// Base implements ValidManifest
func (manifest *Timestamp) Base() *SignedBase {
	return &manifest.SignedBase
}

// Filename implements ValidManifest
func (manifest *Root) Filename() string {
	return "root.json"
}

// Filename implements ValidManifest
func (manifest *Index) Filename() string {
	return "index.json"
}

// Filename implements ValidManifest
func (manifest *Component) Filename() string {
	panic("Unreachable")
}

// Filename implements ValidManifest
func (manifest *Snapshot) Filename() string {
	return "snapshot.json"
}

// Filename implements ValidManifest
func (manifest *Timestamp) Filename() string {
	return "timestamp.json"
}

// GetRole implements ValidManifest
func (manifest *Root) GetRole() *Role {
	return &Role{
		URL:       fmt.Sprintf("/%s", manifest.Base().Filename()),
		Threshold: 3, // root manifest has higher threshold
		//Keys:      &KeyStore{},
	}
}

// GetRole implements ValidManifest
func (manifest *Index) GetRole() *Role {
	return &Role{
		URL:       fmt.Sprintf("/%s", manifest.Base().Filename()),
		Threshold: 1,
		//Keys:      &KeyStore{},
	}
}

// GetRole implements ValidManifest
func (manifest *Component) GetRole() *Role {
	return &Role{
		URL:       fmt.Sprintf("/%s", manifest.Base().Filename()),
		Threshold: 1,
		//Keys:      &KeyStore{},
	}
}

// GetRole implements ValidManifest
func (manifest *Snapshot) GetRole() *Role {
	return &Role{
		URL:       fmt.Sprintf("/%s", manifest.Base().Filename()),
		Threshold: 1,
		//Keys:      &KeyStore{},
	}
}

// GetRole implements ValidManifest
func (manifest *Timestamp) GetRole() *Role {
	return &Role{
		URL:       fmt.Sprintf("/%s", manifest.Base().Filename()),
		Threshold: 1,
		//Keys:      &KeyStore{},
	}
}

// SetVersions sets file versions to the snapshot
func (manifest *Snapshot) SetVersions(manifestList []ValidManifest) *Snapshot {
	if manifest.Meta == nil {
		manifest.Meta = make(map[string]FileVersion)
	}
	for _, m := range manifestList {
		manifest.Meta[m.Base().Filename()] = FileVersion{
			Version: m.Base().Version,
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

// ReadManifest reads a manifest from input and validates it, the result is stored in role, which must be a pointer type.
func ReadManifest(input io.Reader, role ValidManifest, keys *KeyStore) (*Manifest, error) {
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

func readTimestampManifest(input io.Reader, keys *KeyStore) (*Timestamp, error) {
	var ts Timestamp
	_, err := ReadManifest(input, &ts, keys)
	if err != nil {
		return nil, err
	}

	return &ts, nil
}

// signAndWrite creates a manifest and writes it to out.
func signAndWrite(out io.Writer, role ValidManifest) error {
	// TODO sign the result here and make signatures
	_, err := json.Marshal(role)
	if err != nil {
		return err
	}

	manifest := Manifest{
		Signatures: []signature{{
			KeyID: "TODO",
			Sig:   "TODO",
		}},
		Signed: role,
	}

	encoder := json.NewEncoder(out)
	return encoder.Encode(manifest)
}
