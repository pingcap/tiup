package model

import (
	"fmt"

	cjson "github.com/gibson042/canonicaljson-go"
	"github.com/juju/errors"
	"github.com/pingcap-incubator/tiup/pkg/repository/v1manifest"
	"github.com/pingcap-incubator/tiup/pkg/utils"
	"github.com/pingcap-incubator/tiup/server/store"
)

// Model defines operations on the manifests
type Model interface {
	UpdateComponentManifest(component string, manifest *ComponentManifest) error
	UpdateRootManifest(manifest *RootManifest) error
	UpdateIndexManifest(func(*IndexManifest) *IndexManifest) error
	UpdateSnapshotManifest(func(*SnapshotManifest) *SnapshotManifest) error
	UpdateTimestampManifest() error
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
	lastVersion := snap.Signed.Meta[fmt.Sprintf("/%s.json", component)].Version
	if manifest.Signed.Version != lastVersion+1 {
		return ErrorConflict
	}
	if err := m.txn.WriteManifest(fmt.Sprintf("%d.%s.json", manifest.Signed.Version, component), manifest); err != nil {
		return err
	}
	return nil
}

func (m *model) UpdateRootManifest(manifest *RootManifest) error {
	var last RootManifest
	if err := m.txn.ReadManifest("root.json", &last); err != nil {
		return err
	}
	if manifest.Signed.Version != last.Signed.Version+1 {
		return ErrorConflict
	}
	if err := m.txn.WriteManifest("root.json", manifest); err != nil {
		return err
	}
	return m.txn.WriteManifest(fmt.Sprintf("%d.root.json", manifest.Signed.Version), manifest)
}

func (m *model) UpdateIndexManifest(f func(*IndexManifest) *IndexManifest) error {
	snap, err := m.ReadSnapshotManifest()
	if err != nil {
		return err
	}
	lastVersion := snap.Signed.Meta["/index.json"].Version

	var last IndexManifest
	if err := m.txn.ReadManifest(fmt.Sprintf("%d.index.json", lastVersion), &last); err != nil {
		return err
	}
	manifest := f(&last)
	manifest.Signed.Version = last.Signed.Version + 1
	manifest.Signatures, err = sign(manifest.Signed, m.keys[v1manifest.ManifestTypeIndex])
	if err != nil {
		return err
	}

	if err := m.txn.WriteManifest("index.json", manifest); err != nil {
		return err
	}
	return m.txn.WriteManifest(fmt.Sprintf("%d.index.json", manifest.Signed.Version), manifest)
}

func (m *model) UpdateSnapshotManifest(f func(*SnapshotManifest) *SnapshotManifest) error {
	var last SnapshotManifest
	err := m.txn.ReadManifest("snapshot.json", &last)
	if err != nil {
		return err
	}
	manifest := f(&last)
	manifest.Signatures, err = sign(manifest.Signed, m.keys[v1manifest.ManifestTypeSnapshot])
	if err != nil {
		return err
	}

	return m.txn.WriteManifest("snapshot.json", manifest)
}

// ReadSnapshotManifest returns snapshot.json
func (m *model) ReadSnapshotManifest() (*SnapshotManifest, error) {
	var snap SnapshotManifest
	if err := m.txn.ReadManifest("snapshot.json", &snap); err != nil {
		return nil, err
	}
	return &snap, nil
}

func (m *model) UpdateTimestampManifest() error {
	fi, err := m.txn.Stat("snapshot.json")
	if err != nil {
		return err
	}
	reader, err := m.txn.Read("snapshot.json")
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
	err = m.txn.ReadManifest("timestamp.json", &manifest)
	if err != nil {
		return err
	}
	manifest.Signed.Version++
	manifest.Signed.Meta["/snapshot.json"] = v1manifest.FileHash{
		Hashes: map[string]string{
			"sha256": sha256,
		},
		Length: uint(fi.Size()),
	}
	manifest.Signatures, err = sign(manifest.Signed, m.keys[v1manifest.ManifestTypeTimestamp])
	if err != nil {
		return err
	}
	return m.txn.WriteManifest("timestamp.json", &manifest)
}

func sign(signed interface{}, keys ...*v1manifest.KeyInfo) ([]signature, error) {
	payload, err := cjson.Marshal(signed)
	if err != nil {
		return nil, err
	}

	signs := []signature{}
	for _, k := range keys {
		id, err := k.ID()
		if err != nil {
			return nil, errors.Trace(err)
		}
		sign, err := k.Signature(payload)
		if err != nil {
			return nil, errors.Trace(err)
		}
		if err != nil {
			return nil, errors.Trace(err)
		}
		signs = append(signs, signature{
			KeyID: id,
			Sig:   sign,
		})
	}

	return signs, nil
}
