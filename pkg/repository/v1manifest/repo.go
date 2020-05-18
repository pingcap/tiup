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
	"os"
	"path"
	"time"

	cjson "github.com/gibson042/canonicaljson-go"
	"github.com/pingcap/errors"
)

// Init creates and initializes an empty reposityro
func Init(dst, keyDir string, initTime time.Time) error {
	// initial manifests
	manifests := make(map[string]ValidManifest)

	// init the root manifest
	manifests[ManifestTypeRoot] = NewRoot(initTime)

	// init index
	manifests[ManifestTypeIndex] = NewIndex(initTime)

	// snapshot and timestamp are the last two manifests to be initialized
	// init snapshot
	manifests[ManifestTypeSnapshot] = NewSnapshot(initTime).SetVersions(manifests)

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
			manifests[ManifestTypeRoot].(*Root).SetRole(m)
			continue
		}
		// FIXME: log a warning about manifest not found instead of returning error
		return fmt.Errorf("manifest '%s' not initialized porperly", ty)
	}

	keys, err := saveKeys(keyDir, manifests[ManifestTypeRoot].(*Root).Roles)
	if err != nil {
		return err
	}

	return BatchSaveManifests(dst, manifests, keys)
}

func saveKeys(keyDir string, roles map[string]*Role) (map[string][]*KeyInfo, error) {
	privKeys := map[string][]*KeyInfo{}
	for ty, role := range roles {
		for i := 0; i < int(role.Threshold); i++ {
			priv, err := GenKeyInfo()
			if err != nil {
				return nil, err
			}
			privKeys[ty] = append(privKeys[ty], priv)
			pub, err := priv.Public()
			if err != nil {
				return nil, err
			}
			id, err := pub.ID()
			if err != nil {
				// XXX: maybe we can assume no error will be throw here
				return nil, err
			}
			role.Keys[id] = pub

			// Write to key file
			f, err := os.Create(path.Join(keyDir, id[:16]+"-"+ty+".json"))
			if err != nil {
				return nil, err
			}
			defer f.Close()

			if err := json.NewEncoder(f).Encode(priv); err != nil {
				return nil, err
			}
		}
	}
	return privKeys, nil
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

// SignAndWrite creates a manifest and writes it to out.
func SignAndWrite(out io.Writer, role ValidManifest, keys ...*KeyInfo) error {
	payload, err := cjson.Marshal(role)
	if err != nil {
		return err
	}

	signs := []signature{}
	for _, k := range keys {
		if sig, err := k.Signature(payload); err != nil {
			return errors.Trace(err)
		} else if id, err := k.ID(); err != nil {
			return errors.Trace(err)
		} else {
			signs = append(signs, signature{
				KeyID: id,
				Sig:   sig,
			})
		}
	}

	manifest := Manifest{
		Signatures: signs,
		Signed:     role,
	}

	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "\t")
	return encoder.Encode(manifest)
}

// SetVersions sets file versions to the snapshot
func (manifest *Snapshot) SetVersions(manifestList map[string]ValidManifest) *Snapshot {
	if manifest.Meta == nil {
		manifest.Meta = make(map[string]FileVersion)
	}
	for _, m := range manifestList {
		manifest.Meta[m.Filename()] = FileVersion{
			Version: m.Base().Version,
		}
	}
	return manifest
}

// SetRole populates role list in the root manifest
func (manifest *Root) SetRole(m ValidManifest) error {
	if manifest.Roles == nil {
		manifest.Roles = make(map[string]*Role)
	}

	manifest.Roles[m.Base().Ty] = &Role{
		URL:       fmt.Sprintf("/%s", m.Filename()),
		Threshold: ManifestsConfig[m.Base().Ty].Threshold,
		Keys:      make(map[string]*KeyInfo),
	}

	return nil
}
