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
	"strings"
	"time"

	cjson "github.com/gibson042/canonicaljson-go"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/repository/store"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/pingcap/tiup/pkg/version"
	"go.uber.org/zap"
)

// Backend defines operations on the manifests
type Backend interface {
	// Publish push a new component to mirror or modify an exists component
	Publish(manifest *v1manifest.Manifest, info ComponentInfo) error
	// Introduce add a new owner to mirror
	Grant(id, name string, key *v1manifest.KeyInfo) error
	// Rotate update root manifest
	Rotate(manifest *v1manifest.Manifest) error
}

type model struct {
	txn  store.FsTxn
	keys map[string]*v1manifest.KeyInfo
}

// New returns a object implemented Backend
func New(txn store.FsTxn, keys map[string]*v1manifest.KeyInfo) Backend {
	return &model{txn, keys}
}

// Grant implements Backend
func (m *model) Grant(id, name string, key *v1manifest.KeyInfo) error {
	initTime := time.Now()

	keyID, err := key.ID()
	if err != nil {
		return err
	}

	return utils.RetryUntil(func() error {
		var indexFileVersion *v1manifest.FileVersion
		if err := m.updateIndexManifest(initTime, func(im *v1manifest.Manifest) (*v1manifest.Manifest, error) {
			signed := im.Signed.(*v1manifest.Index)

			for oid, owner := range signed.Owners {
				if oid == id {
					return nil, errors.Errorf("owner %s exists", id)
				}

				for kid := range owner.Keys {
					if kid == keyID {
						return nil, errors.Errorf("key %s exists", keyID)
					}
				}
			}

			signed.Owners[id] = v1manifest.Owner{
				Name: name,
				Keys: map[string]*v1manifest.KeyInfo{
					keyID: key,
				},
				// TODO: support configable threshold
				Threshold: 1,
			}

			indexFileVersion = &v1manifest.FileVersion{Version: signed.Version + 1}
			return im, nil
		}); err != nil {
			return err
		}

		if indexFi, err := m.txn.Stat(fmt.Sprintf("%d.index.json", indexFileVersion.Version)); err == nil {
			indexFileVersion.Length = uint(indexFi.Size())
		} else {
			return err
		}

		if err := m.updateSnapshotManifest(initTime, func(om *v1manifest.Manifest) *v1manifest.Manifest {
			signed := om.Signed.(*v1manifest.Snapshot)
			if indexFileVersion != nil {
				signed.Meta[v1manifest.ManifestURLIndex] = *indexFileVersion
			}
			return om
		}); err != nil {
			return err
		}

		// Update timestamp.json and signature
		if err := m.updateTimestampManifest(initTime); err != nil {
			return err
		}

		return m.txn.Commit()
	}, func(err error) bool {
		return err == store.ErrorFsCommitConflict && m.txn.ResetManifest() == nil
	})
}

// Rotate implements Backend
func (m *model) Rotate(manifest *v1manifest.Manifest) error {
	initTime := time.Now()
	root, ok := manifest.Signed.(*v1manifest.Root)
	if !ok {
		return ErrorWrongManifestType
	}

	return utils.RetryUntil(func() error {
		rm, err := m.readRootManifest()
		if err != nil {
			return err
		}

		if err := verifyRootManifest(rm, manifest); err != nil {
			return err
		}

		// write new 'root.json' file with verion prefix
		manifestFilename := fmt.Sprintf("%d.root.json", root.Version)
		if err := m.txn.WriteManifest(manifestFilename, manifest); err != nil {
			return err
		}
		/* not yet update the 'root.json' without version prefix, as we don't
		 * have a '1.root.json', so the 'root.json' is playing the role of initial
		 * '1.root.json', clients are updating to the latest 'n.root.json' no
		 * matter older ones are expired or not
		 * maybe we could update the 'root.json' some day when we have many many
		 * versions of root.json available and the updating process from old clients
		 * are causing performance issues
		 */
		// if err := m.txn.WriteManifest("root.json", manifest); err != nil {
		// 	return err
		// }

		fi, err := m.txn.Stat(manifestFilename)
		if err != nil {
			return err
		}

		if err := m.updateSnapshotManifest(initTime, func(om *v1manifest.Manifest) *v1manifest.Manifest {
			signed := om.Signed.(*v1manifest.Snapshot)
			signed.Meta[v1manifest.ManifestURLRoot] = v1manifest.FileVersion{
				Version: root.Version,
				Length:  uint(fi.Size()),
			}
			return om
		}); err != nil {
			return err
		}

		// Update timestamp.json and signature
		if err := m.updateTimestampManifest(initTime); err != nil {
			return err
		}

		return m.txn.Commit()
	}, func(err error) bool {
		return err == store.ErrorFsCommitConflict && m.txn.ResetManifest() == nil
	})
}

// Publish implements Backend
func (m *model) Publish(manifest *v1manifest.Manifest, info ComponentInfo) error {
	signed := manifest.Signed.(*v1manifest.Component)
	initTime := time.Now()
	pf := func() error {
		// Write the component manifest (component.json)
		if err := m.updateComponentManifest(manifest); err != nil {
			return err
		}

		// Update snapshot.json and signature
		fi, err := m.txn.Stat(fmt.Sprintf("%d.%s.json", signed.Version, signed.ID))
		if err != nil {
			return err
		}

		var indexFileVersion *v1manifest.FileVersion
		var owner *v1manifest.Owner
		if err := m.updateIndexManifest(initTime, func(im *v1manifest.Manifest) (*v1manifest.Manifest, error) {
			// We only update index.json when it's a new component
			// or the yanked, standalone, hidden fileds changed,
			// or the owner of component changed
			var (
				compItem  v1manifest.ComponentItem
				compExist bool
			)

			componentName := signed.ID
			signed := im.Signed.(*v1manifest.Index)
			if compItem, compExist = signed.Components[componentName]; compExist {
				// Find the owner of target component
				var o v1manifest.Owner
				if info.OwnerID() != "" {
					o = signed.Owners[info.OwnerID()]
				} else {
					o = signed.Owners[compItem.Owner]
				}
				owner = &o
				if info.Yanked() == nil &&
					info.Hidden() == nil &&
					info.Standalone() == nil &&
					info.OwnerID() == "" {
					// No changes on index.json
					return nil, nil
				}
			} else {
				var ownerID string
				// The component is a new component, so the owner is whoever first create it.
				for _, sk := range manifest.Signatures {
					if ownerID, owner = findKeyOwnerFromIndex(signed, sk.KeyID); owner != nil {
						break
					}
				}
				compItem = v1manifest.ComponentItem{
					Owner: ownerID,
					URL:   fmt.Sprintf("/%s.json", componentName),
				}
			}
			if info.Yanked() != nil {
				compItem.Yanked = *info.Yanked()
			}
			if info.Hidden() != nil {
				compItem.Hidden = *info.Hidden()
			}
			if info.Standalone() != nil {
				compItem.Standalone = *info.Standalone()
			}
			if info.OwnerID() != "" {
				compItem.Owner = info.OwnerID()
			}

			signed.Components[componentName] = compItem
			indexFileVersion = &v1manifest.FileVersion{Version: signed.Version + 1}
			return im, nil
		}); err != nil {
			return err
		}

		if err := verifyComponentManifest(owner, manifest); err != nil {
			return err
		}

		if indexFileVersion != nil {
			if indexFi, err := m.txn.Stat(fmt.Sprintf("%d.index.json", indexFileVersion.Version)); err == nil {
				indexFileVersion.Length = uint(indexFi.Size())
			} else {
				return err
			}
		}

		if err := m.updateSnapshotManifest(initTime, func(om *v1manifest.Manifest) *v1manifest.Manifest {
			componentName := signed.ID
			manifestVersion := signed.Version
			signed := om.Signed.(*v1manifest.Snapshot)
			if indexFileVersion != nil {
				signed.Meta[v1manifest.ManifestURLIndex] = *indexFileVersion
			}
			signed.Meta[fmt.Sprintf("/%s.json", componentName)] = v1manifest.FileVersion{
				Version: manifestVersion,
				Length:  uint(fi.Size()),
			}
			return om
		}); err != nil {
			return err
		}

		// Update timestamp.json and signature
		if err := m.updateTimestampManifest(initTime); err != nil {
			return err
		}

		if info.Filename() != "" {
			if err := m.checkAndWrite(signed, info); err != nil {
				return err
			}
			if signed.ID == version.TiUPVerName {
				if err := m.copyTiUP(info.Filename()); err != nil {
					return err
				}
			}
		}
		return m.txn.Commit()
	}

	return utils.RetryUntil(pf, func(err error) bool {
		return err == store.ErrorFsCommitConflict && m.txn.ResetManifest() == nil
	})
}

func (m *model) copyTiUP(origin string) error {
	xs := strings.Split(origin, "-")
	if len(xs) < 4 {
		return ErrorWrongFileName
	}
	// convert
	//	`tiup-${version}-linux-arm64.tar.gz`  -> `tiup-linux-arm64.tar.gz`
	//  `tiup-v1.4.0-darwin-amd64.tar.gz` -> `tiup-darwin-amd64.tar.gz`
	//  `tiup-v1.4.0-r13-gcd19b75+staging-darwin-amd64.tar.gz` -> `tiup-darwin-amd64.tar.gz`
	tiupTar := strings.Join(append(xs[:1], xs[len(xs)-2:]...), "-")
	reader, err := m.txn.Read(origin)
	if err != nil {
		return err
	}
	defer reader.Close()
	return m.txn.Write(tiupTar, reader)
}

func (m *model) checkAndWrite(manifest *v1manifest.Component, info ComponentData) error {
	fname := info.Filename()
	for _, plat := range manifest.Platforms {
		for _, vi := range plat {
			if vi.URL[1:] == fname {
				if err := m.txn.Write(fname, info); err != nil {
					return err
				}
				reader, err := m.txn.Read(fname)
				if err != nil {
					return err
				}
				defer reader.Close()
				if err := utils.CheckSHA256(reader, vi.Hashes["sha256"]); err == nil {
					return nil
				}
				return ErrorWrongChecksum
			}
		}
	}
	return ErrorWrongFileName
}

func findKeyOwnerFromIndex(signed *v1manifest.Index, keyID string) (string, *v1manifest.Owner) {
	for on := range signed.Owners {
		for k := range signed.Owners[on].Keys {
			if k == keyID {
				o := signed.Owners[on]
				return on, &o
			}
		}
	}
	return "", nil
}

func (m *model) updateComponentManifest(manifest *v1manifest.Manifest) error {
	signed := manifest.Signed.(*v1manifest.Component)
	snap, err := m.readSnapshotManifest()
	if err != nil {
		return err
	}
	snapSigned := snap.Signed.(*v1manifest.Snapshot)
	lastVersion := snapSigned.Meta["/"+signed.Filename()].Version
	if signed.Version != lastVersion+1 {
		zap.L().Debug(
			"Component version not expected",
			zap.Uint("expected", lastVersion+1),
			zap.Uint("got", signed.Version),
		)
		return ErrorConflict
	}
	return m.txn.WriteManifest(fmt.Sprintf("%d.%s.json", signed.Version, signed.ID), manifest)
}

func (m *model) updateIndexManifest(initTime time.Time, f func(*v1manifest.Manifest) (*v1manifest.Manifest, error)) error {
	snap, err := m.readSnapshotManifest()
	if err != nil {
		return err
	}
	snapSigned := snap.Signed.(*v1manifest.Snapshot)
	lastVersion := snapSigned.Meta[v1manifest.ManifestURLIndex].Version

	last, err := m.txn.ReadManifest(fmt.Sprintf("%d.index.json", lastVersion), &v1manifest.Index{})
	if err != nil {
		return err
	}
	manifest, err := f(last)
	if err != nil {
		return err
	}
	if manifest == nil {
		return nil
	}
	signed := manifest.Signed.(*v1manifest.Index)
	v1manifest.RenewManifest(signed, initTime)
	manifest.Signatures, err = m.sign(manifest.Signed)
	if err != nil {
		return err
	}

	return m.txn.WriteManifest(fmt.Sprintf("%d.index.json", signed.Version), manifest)
}

func (m *model) updateSnapshotManifest(initTime time.Time, f func(*v1manifest.Manifest) *v1manifest.Manifest) error {
	last, err := m.txn.ReadManifest(v1manifest.ManifestFilenameSnapshot, &v1manifest.Snapshot{})
	if err != nil {
		return err
	}
	manifest := f(last)
	if manifest == nil {
		return nil
	}
	v1manifest.RenewManifest(manifest.Signed, initTime)
	manifest.Signatures, err = m.sign(manifest.Signed)
	if err != nil {
		return err
	}

	return m.txn.WriteManifest(v1manifest.ManifestFilenameSnapshot, manifest)
}

// readSnapshotManifest returns snapshot.json
func (m *model) readSnapshotManifest() (*v1manifest.Manifest, error) {
	return m.txn.ReadManifest(v1manifest.ManifestFilenameSnapshot, &v1manifest.Snapshot{})
}

// readRootManifest returns the latest root.json
func (m *model) readRootManifest() (*v1manifest.Manifest, error) {
	root, err := m.txn.ReadManifest(v1manifest.ManifestFilenameRoot, &v1manifest.Root{})
	if err != nil {
		return root, err
	}

	for {
		file := fmt.Sprintf("%d.%s", root.Signed.Base().Version+1, root.Signed.Filename())
		last, err := m.txn.ReadManifest(file, &v1manifest.Root{})
		if err != nil {
			return root, nil
		}
		root = last
	}
}

func (m *model) updateTimestampManifest(initTime time.Time) error {
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

	manifest, err := m.txn.ReadManifest(v1manifest.ManifestFilenameTimestamp, &v1manifest.Timestamp{})
	if err != nil {
		return err
	}
	signed := manifest.Signed.(*v1manifest.Timestamp)
	signed.Meta[v1manifest.ManifestURLSnapshot] = v1manifest.FileHash{
		Hashes: map[string]string{
			v1manifest.SHA256: sha256,
		},
		Length: uint(fi.Size()),
	}
	v1manifest.RenewManifest(manifest.Signed, initTime)
	manifest.Signatures, err = m.sign(manifest.Signed)
	if err != nil {
		return err
	}

	return m.txn.WriteManifest(v1manifest.ManifestFilenameTimestamp, manifest)
}

func (m *model) sign(signed v1manifest.ValidManifest) ([]v1manifest.Signature, error) {
	payload, err := cjson.Marshal(signed)
	if err != nil {
		return nil, err
	}

	rm, err := m.readRootManifest()
	if err != nil {
		return nil, err
	}
	root := rm.Signed.(*v1manifest.Root)

	signs := []v1manifest.Signature{}
	for _, pubKey := range root.Roles[signed.Base().Ty].Keys {
		id, err := pubKey.ID()
		if err != nil {
			return nil, err
		}

		privKey := m.keys[id]
		if privKey == nil {
			return nil, ErrorMissingKey
		}

		sign, err := privKey.Signature(payload)
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

func verifyComponentManifest(owner *v1manifest.Owner, m *v1manifest.Manifest) error {
	if owner == nil {
		return ErrorMissingOwner
	}

	payload, err := cjson.Marshal(m.Signed)
	if err != nil {
		return err
	}

	for _, s := range m.Signatures {
		k := owner.Keys[s.KeyID]
		if k == nil {
			continue
		}

		if err := k.Verify(payload, s.Sig); err == nil {
			return nil
		}
	}

	return ErrorWrongSignature
}

func verifyRootManifest(oldM *v1manifest.Manifest, newM *v1manifest.Manifest) error {
	newRoot := newM.Signed.(*v1manifest.Root)
	newKeys := set.NewStringSet()
	payload, err := cjson.Marshal(newM.Signed)
	if err != nil {
		return err
	}
	for _, s := range newM.Signatures {
		id := s.KeyID
		k := newRoot.Roles[v1manifest.ManifestTypeRoot].Keys[id]
		if err := k.Verify(payload, s.Sig); err == nil {
			newKeys.Insert(s.KeyID)
		}
	}

	oldRoot := oldM.Signed.(*v1manifest.Root)
	oldKeys := set.NewStringSet()
	for id := range oldRoot.Roles[v1manifest.ManifestTypeRoot].Keys {
		oldKeys.Insert(id)
	}

	if len(oldKeys.Intersection(newKeys).Slice()) < int(oldRoot.Roles[v1manifest.ManifestTypeRoot].Threshold) {
		return errors.Annotatef(ErrorWrongSignature,
			"need %d valid signatures, only got %d",
			oldRoot.Roles[v1manifest.ManifestTypeRoot].Threshold,
			len(oldKeys.Intersection(newKeys).Slice()),
		)
	}

	if newRoot.Version != oldRoot.Version+1 {
		return errors.Annotatef(ErrorWrongManifestVersion, "expect %d, got %d", oldRoot.Version+1, newRoot.Version)
	}

	return nil
}
