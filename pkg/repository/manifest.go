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

type signedBase struct {
	Ty          string `json:"_type"`
	SpecVersion string `json:"spec_version"`
	Expires     string `json:"expires"`
	// 0 => no version specified
	Version uint `json:"version"`
}

type manifest struct {
	Signatures []signature   `json:"signatures"`
	Signed     ValidManifest `json:"signed"`
}

// ValidManifest is a manifest which includes signedBase and can be validated.
type ValidManifest interface {
	isValid() error
	base() *signedBase
}

var types = map[string]struct{}{"root": {}, "index": {}, "component": {}, "snapshot": {}, "timestamp": {}}
var knownVersions = map[string]struct{}{"0.1.0": {}}

type timestamp struct {
	signedBase
	Meta map[string]fileHash `json:"meta"`
}

type fileHash struct {
	Hashes map[string]string `json:"hashes"`
	Length uint              `json:"length"`
}

// verifySignature ensures that each signature in manifest::signatures is a valid signature of manifest::signed.
func (manifest *manifest) verifySignature(keys *keyStore) error {
	// TODO
	return nil
}

func (s *signedBase) expiryTime() (time.Time, error) {
	return time.Parse(time.RFC3339, s.Expires)
}

// isValid checks if s is valid manifest metadata.
func (s *signedBase) isValid() error {
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

func (ts *timestamp) isValid() error {
	snapshot, ok := ts.Meta["snapshot.json"]
	if !ok {
		return errors.New("timestamp manifest is missing entry for snapshot.json")
	}
	if len(ts.Meta) > 1 {
		return errors.New("timestamp manifest has too many entries in `meta`")
	}
	if len(snapshot.Hashes) == 0 {
		return errors.New("timestamp manifest missing hash for snapshot.json")
	}
	return nil
}

func (ts *timestamp) base() *signedBase {
	return &ts.signedBase
}

func readManifest(input io.Reader, role ValidManifest, keys *keyStore) error {
	decoder := json.NewDecoder(input)
	var m manifest
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

	err = m.Signed.base().isValid()
	if err != nil {
		return err
	}

	return m.Signed.isValid()
}

func readTimestampManifest(input io.Reader, keys *keyStore) (*timestamp, error) {
	var ts timestamp
	err := readManifest(input, &ts, keys)
	if err != nil {
		return nil, err
	}

	return &ts, nil
}

// writeManifest creates a manifest and writes it to out.
func writeManifest(out io.Writer, role ValidManifest) error {
	// TODO sign the result here and make signatures
	_, err := json.Marshal(role)
	if err != nil {
		return err
	}

	manifest := manifest{
		Signatures: []signature{{
			KeyID: "TODO",
			Sig:   "TODO",
		}},
		Signed: role,
	}

	encoder := json.NewEncoder(out)
	return encoder.Encode(manifest)
}
