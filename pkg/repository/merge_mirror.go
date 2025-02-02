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
	"fmt"
	"strings"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/repository/model"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/pingcap/tiup/pkg/utils"
)

type diffItem struct {
	name          string
	componentItem v1manifest.ComponentItem
	versionItem   v1manifest.VersionItem
	version       string
	os            string
	arch          string
	desc          string
}

// returns component that exists in addition but not in base
func diffMirror(base, addition Mirror) ([]diffItem, error) {
	baseIndex, err := fetchIndexManifestFromMirror(base)
	if err != nil {
		return nil, err
	}
	additionIndex, err := fetchIndexManifestFromMirror(addition)
	if err != nil {
		return nil, err
	}

	items := []diffItem{}

	baseComponents := baseIndex.ComponentListWithYanked()
	additionComponents := additionIndex.ComponentList()

	for name, comp := range additionComponents {
		if baseComponents[name].Yanked {
			continue
		}

		baseComponent, err := fetchComponentManifestFromMirror(base, name)
		if err != nil {
			return nil, err
		}
		additionComponent, err := fetchComponentManifestFromMirror(addition, name)
		if err != nil {
			return nil, err
		}

		items = append(items, component2Diff(name, baseComponents[name], baseComponent, comp, additionComponent)...)
	}

	return items, nil
}

func component2Diff(name string, baseItem v1manifest.ComponentItem, baseManifest *v1manifest.Component,
	additionItem v1manifest.ComponentItem, additionManifest *v1manifest.Component) []diffItem {
	items := []diffItem{}

	for plat := range additionManifest.Platforms {
		versions := additionManifest.VersionList(plat)
		for ver, verinfo := range versions {
			// Don't merge nightly this time
			if utils.Version(ver).IsNightly() {
				continue
			}

			// this version not exits in base
			if baseManifest.VersionList(plat)[ver].URL == "" {
				osArch := strings.Split(plat, "/")
				if len(osArch) != 2 {
					continue
				}

				item := diffItem{
					name:          name,
					componentItem: baseItem,
					versionItem:   verinfo,
					version:       ver,
					os:            osArch[0],
					arch:          osArch[1],
					desc:          additionManifest.Description,
				}

				if baseItem.URL == "" {
					item.componentItem = additionItem
				}
				items = append(items, item)
			}
		}
	}

	return items
}

// MergeMirror merges two or more mirrors
func MergeMirror(keys map[string]*v1manifest.KeyInfo, base Mirror, additions ...Mirror) error {
	ownerKeys, err := mapOwnerKeys(base, keys)
	if err != nil {
		return err
	}

	for _, addition := range additions {
		diffs, err := diffMirror(base, addition)
		if err != nil {
			return err
		}

		for _, diff := range diffs {
			if len(ownerKeys[diff.componentItem.Owner]) == 0 {
				return errors.Errorf("missing owner keys for owner %s on component %s", diff.componentItem.Owner, diff.name)
			}

			comp, err := fetchComponentManifestFromMirror(base, diff.name)
			if err != nil {
				return err
			}

			comp = UpdateManifestForPublish(comp, diff.name, diff.version, diff.versionItem.Entry, diff.os, diff.arch, diff.desc, diff.versionItem.FileHash)
			manifest, err := v1manifest.SignManifest(comp, ownerKeys[diff.componentItem.Owner]...)
			if err != nil {
				return err
			}

			resource := strings.TrimPrefix(diff.versionItem.URL, "/")
			tarfile, err := addition.Fetch(resource, 0)
			if err != nil {
				return err
			}
			defer tarfile.Close()

			publishInfo := &model.PublishInfo{
				ComponentData: &model.TarInfo{Reader: tarfile, Name: resource},
				Stand:         &diff.componentItem.Standalone,
				Hide:          &diff.componentItem.Hidden,
			}

			if err := base.Publish(manifest, publishInfo); err != nil {
				return err
			}
		}
	}
	return nil
}

func fetchComponentManifestFromMirror(mirror Mirror, component string) (*v1manifest.Component, error) {
	r, err := mirror.Fetch(v1manifest.ManifestFilenameSnapshot, 0)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	snap := v1manifest.Snapshot{}
	if _, err := v1manifest.ReadNoVerify(r, &snap); err != nil {
		return nil, err
	}

	v := snap.Meta[fmt.Sprintf("/%s.json", component)].Version
	if v == 0 {
		// nil means that the component manifest not found
		return nil, nil
	}

	r, err = mirror.Fetch(fmt.Sprintf("%d.%s.json", v, component), 0)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	role := v1manifest.Component{}
	// TODO: this time we just assume the addition mirror is trusted
	if _, err := v1manifest.ReadNoVerify(r, &role); err != nil {
		return nil, err
	}

	return &role, nil
}

func fetchIndexManifestFromMirror(mirror Mirror) (*v1manifest.Index, error) {
	r, err := mirror.Fetch(v1manifest.ManifestFilenameSnapshot, 0)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	snap := v1manifest.Snapshot{}
	if _, err := v1manifest.ReadNoVerify(r, &snap); err != nil {
		return nil, err
	}

	indexVersion := snap.Meta[v1manifest.ManifestURLIndex].Version
	if indexVersion == 0 {
		return nil, errors.Errorf("missing index manifest in base mirror")
	}

	r, err = mirror.Fetch(fmt.Sprintf("%d.%s", indexVersion, v1manifest.ManifestFilenameIndex), 0)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	index := v1manifest.Index{}
	if _, err := v1manifest.ReadNoVerify(r, &index); err != nil {
		return nil, err
	}

	return &index, nil
}

// the keys in param is keyID -> KeyInfo, we should map it to ownerID -> KeyInfoList
func mapOwnerKeys(base Mirror, keys map[string]*v1manifest.KeyInfo) (map[string][]*v1manifest.KeyInfo, error) {
	index, err := fetchIndexManifestFromMirror(base)
	if err != nil {
		return nil, err
	}

	keyList := map[string][]*v1manifest.KeyInfo{}
	for ownerID, owner := range index.Owners {
		for keyID := range owner.Keys {
			if key := keys[keyID]; key != nil {
				keyList[ownerID] = append(keyList[ownerID], key)
			}
		}
		if len(keyList[ownerID]) < owner.Threshold {
			// We set keys of this owner to empty because we can't clone components belong to this owner
			keyList[ownerID] = nil
		}
	}
	return keyList, nil
}

// UpdateManifestForPublish set corresponding field for component manifest
func UpdateManifestForPublish(m *v1manifest.Component,
	name, ver, entry, os, arch, desc string,
	filehash v1manifest.FileHash) *v1manifest.Component {
	initTime := time.Now()

	// update manifest
	if m == nil {
		m = v1manifest.NewComponent(name, desc, initTime)
	} else {
		v1manifest.RenewManifest(m, initTime)
		if desc != "" {
			m.Description = desc
		}
	}

	if utils.Version(ver).IsNightly() {
		m.Nightly = ver
	}

	// Remove history nightly
	for plat := range m.Platforms {
		for v := range m.Platforms[plat] {
			if strings.Contains(v, utils.NightlyVersionAlias) && v != m.Nightly {
				delete(m.Platforms[plat], v)
			}
		}
	}

	platformStr := fmt.Sprintf("%s/%s", os, arch)
	if m.Platforms[platformStr] == nil {
		m.Platforms[platformStr] = map[string]v1manifest.VersionItem{}
	}

	m.Platforms[platformStr][ver] = v1manifest.VersionItem{
		Entry:    entry,
		Released: initTime.Format(time.RFC3339),
		URL:      fmt.Sprintf("/%s-%s-%s-%s.tar.gz", name, ver, os, arch),
		FileHash: filehash,
	}

	return m
}
