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
}

// ty is type information about a manifest
type ty struct {
	filename  string
	versioned bool
}

var types = map[string]ty{
	"root":      {"root.json", true},
	"index":     {"index.json", true},
	"component": {"", true},
	"snapshot":  {"snapshot.json", false},
	"timestamp": {"timestamp.json", false},
}
var knownVersions = map[string]struct{}{"0.1.0": {}}

// Root manifest
type Root struct {
	SignedBase
	// TODO
}

// Index manifest
type Index struct {
	SignedBase
	// TODO
}

// Component manifest
type Component struct {
	SignedBase
	// TODO
}

// Snapshot manifest
type Snapshot struct {
	SignedBase
	// TODO
}

// Timestamp manifest
type Timestamp struct {
	SignedBase
	Meta map[string]fileHash `json:"meta"`
}

type fileHash struct {
	Hashes map[string]string `json:"hashes"`
	Length uint              `json:"length"`
}

// verifySignature ensures that each signature in manifest::signatures is a valid signature of manifest::signed.
func (manifest *Manifest) verifySignature(keys *KeyStore) error {
	// TODO
	return nil
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

// ReadManifest reads a manifest from input and validates it, the result is stored in role, which must be a pointer type.
func ReadManifest(input io.Reader, role ValidManifest, keys *KeyStore) error {
	decoder := json.NewDecoder(input)
	var m Manifest
	m.Signed = role
	err := decoder.Decode(&m)
	if err != nil {
		return err
	}

	if len(m.Signatures) == 0 {
		return errors.New("no signatures supplied in manifest")
	}

	err = m.verifySignature(keys)
	if err != nil {
		return err
	}

	err = m.Signed.Base().isValid()
	if err != nil {
		return err
	}

	return m.Signed.isValid()
}

func readTimestampManifest(input io.Reader, keys *KeyStore) (*Timestamp, error) {
	var ts Timestamp
	err := ReadManifest(input, &ts, keys)
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
