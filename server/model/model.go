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

package model

import (
	"fmt"
	"time"

	cjson "github.com/gibson042/canonicaljson-go"
	"github.com/juju/errors"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/pingcap/tiup/server/store"
)

// Model defines operations on the manifests
type Model interface {
	UpdateComponentManifest(component string, manifest *ComponentManifest) error
	UpdateRootManifest(manifest *RootManifest) error
	UpdateIndexManifest(time.Time, func(*IndexManifest) *IndexManifest) error
	UpdateSnapshotManifest(time.Time, func(*SnapshotManifest) *SnapshotManifest) error
	UpdateTimestampManifest(time.Time) error
}

type model struct {
	txn  store.FsTxn
	keys map[string]*v1manifest.KeyInfo
}

// New returns a object implemented Model
func New(txn store.FsTxn, keys map[string]*v1manifest.KeyInfo) Model {
	return &model{txn, keys}
}

func (m *model) UpdateComponentManifest(component string, manifest *ComponentManifest) error {
	snap, err := m.ReadSnapshotManifest()
	if err != nil {
		return err
	}
	lastVersion := snap.Signed.Meta["/"+manifest.Signed.Filename()].Version
	if manifest.Signed.Version != lastVersion+1 {
		log.Debugf("Component version not expected, expect %d, got %d", lastVersion+1, manifest.Signed.Version)
		return ErrorConflict
	}
	return m.txn.WriteManifest(fmt.Sprintf("%d.%s.json", manifest.Signed.Version, component), manifest)
}

func (m *model) UpdateRootManifest(manifest *RootManifest) error {
	var last RootManifest
	if err := m.txn.ReadManifest(v1manifest.ManifestFilenameRoot, &last); err != nil {
		return err
	}
	if manifest.Signed.Version != last.Signed.Version+1 {
		return ErrorConflict
	}
	if err := m.txn.WriteManifest(v1manifest.ManifestFilenameRoot, manifest); err != nil {
		return err
	}

	return m.txn.WriteManifest(fmt.Sprintf("%d.root.json", manifest.Signed.Version), manifest)
}

func (m *model) UpdateIndexManifest(initTime time.Time, f func(*IndexManifest) *IndexManifest) error {
	snap, err := m.ReadSnapshotManifest()
	if err != nil {
		return err
	}
	lastVersion := snap.Signed.Meta[v1manifest.ManifestURLIndex].Version

	var last IndexManifest
	if err := m.txn.ReadManifest(fmt.Sprintf("%d.index.json", lastVersion), &last); err != nil {
		return err
	}
	manifest := f(&last)
	if manifest == nil {
		return nil
	}
	manifest.Signed.Version = last.Signed.Version + 1
	v1manifest.RenewManifest(&manifest.Signed, initTime)
	manifest.Signatures, err = sign(manifest.Signed, m.keys[v1manifest.ManifestTypeIndex])
	if err != nil {
		return err
	}

	return m.txn.WriteManifest(fmt.Sprintf("%d.index.json", manifest.Signed.Version), manifest)
}

func (m *model) UpdateSnapshotManifest(initTime time.Time, f func(*SnapshotManifest) *SnapshotManifest) error {
	var last SnapshotManifest
	err := m.txn.ReadManifest(v1manifest.ManifestFilenameSnapshot, &last)
	if err != nil {
		return err
	}
	manifest := f(&last)
	if manifest == nil {
		return nil
	}
	v1manifest.RenewManifest(&manifest.Signed, initTime)
	manifest.Signatures, err = sign(manifest.Signed, m.keys[v1manifest.ManifestTypeSnapshot])
	if err != nil {
		return err
	}

	return m.txn.WriteManifest(v1manifest.ManifestFilenameSnapshot, manifest)
}

// ReadSnapshotManifest returns snapshot.json
func (m *model) ReadSnapshotManifest() (*SnapshotManifest, error) {
	var snap SnapshotManifest
	if err := m.txn.ReadManifest(v1manifest.ManifestFilenameSnapshot, &snap); err != nil {
		return nil, err
	}
	return &snap, nil
}

// ReadRootManifest returns root.json
func (m *model) ReadRootManifest() (*RootManifest, error) {
	var root RootManifest
	if err := m.txn.ReadManifest(v1manifest.ManifestFilenameRoot, &root); err != nil {
		return nil, err
	}
	return &root, nil
}

func (m *model) UpdateTimestampManifest(initTime time.Time) error {
	fi, err := m.txn.Stat(v1manifest.ManifestFilenameSnapshot)
	if err != nil {
		return err
	}
	reader, err := m.txn.Read(v1manifest.ManifestFilenameSnapshot)
	if err != nil {
		return err
	}
	sha256, err := utils.SHA256(reader)
	if err != nil {
		reader.Close()
		return err
	}
	reader.Close()

	var manifest TimestampManifest
	err = m.txn.ReadManifest(v1manifest.ManifestFilenameTimestamp, &manifest)
	if err != nil {
		return err
	}
	manifest.Signed.Version++
	manifest.Signed.Meta[v1manifest.ManifestURLSnapshot] = v1manifest.FileHash{
		Hashes: map[string]string{
			v1manifest.SHA256: sha256,
		},
		Length: uint(fi.Size()),
	}
	v1manifest.RenewManifest(&manifest.Signed, initTime)
	manifest.Signatures, err = sign(manifest.Signed, m.keys[v1manifest.ManifestTypeTimestamp])
	if err != nil {
		return err
	}

	return m.txn.WriteManifest(v1manifest.ManifestFilenameTimestamp, &manifest)
}

func sign(signed interface{}, keys ...*v1manifest.KeyInfo) ([]v1manifest.Signature, error) {
	payload, err := cjson.Marshal(signed)
	if err != nil {
		return nil, err
	}

	signs := []v1manifest.Signature{}
	for _, k := range keys {
		if k == nil {
			return nil, ErrorMissingKey
		}
		id, err := k.ID()
		if err != nil {
			return nil, errors.Trace(err)
		}
		sign, err := k.Signature(payload)
		if err != nil {
			return nil, errors.Trace(err)
		}
		signs = append(signs, v1manifest.Signature{
			KeyID: id,
			Sig:   sign,
		})
	}

	return signs, nil
}
