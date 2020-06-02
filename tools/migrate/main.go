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

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/repository"
	ru "github.com/pingcap/tiup/pkg/repository/utils"
	"github.com/pingcap/tiup/pkg/repository/v0manifest"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/pingcap/tiup/pkg/utils"
	tiupver "github.com/pingcap/tiup/pkg/version"
	"github.com/spf13/cobra"
)

var signedRoot bool // generate keys and sign root in new command

func main() {
	cmd := &cobra.Command{
		Use:           "migrate <command>",
		Short:         "Migrate a repository from v0 format to v1 manifests",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	newCmd := &cobra.Command{
		Use:           "new <src-dir> <dst-dir>",
		Short:         "Generate new manifest base on exists ola manifest",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return cmd.Help()
			}

			return migrate(args[0], args[1], false)
		},
	}

	newCmd.Flags().BoolVar(&signedRoot, "signed-root", false, "generate keys and sign root in new command")

	var isPublicKey bool
	updateCmd := &cobra.Command{
		Use:   "update <dir> [signed-root.json ...]",
		Short: "Rehash root.json and update snapshot.json, timestamp.json in the repo",
		Long: `Rehash root.json and update snapshot.json, timestamp.json in the repo.
If one or more signed root.json files are specified, the signatures
from them are added to the root.json before updating.`,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			return update(args[0], args[1:], isPublicKey)
		},
	}

	updateCmd.Flags().BoolVar(&isPublicKey, "public", false, "Indicate that inputs are public keys to be added to key list, not signatures")

	rehashCmd := &cobra.Command{
		Use:   "rehash <src-dir> <dst-dir>",
		Short: "Rehash tarballs and update manifests in the repo",
		Long: `Rehash tarballs in src-dir and update manifests in the dst-dir repo.
A already populated repo should be in dst-dir, and the root.json will not be changed.`,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return cmd.Help()
			}
			return migrate(args[0], args[1], true)
		},
	}

	cmd.AddCommand(
		newCmd,
		updateCmd,
		rehashCmd,
	)

	if err := cmd.Execute(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}

func readManifest0(srcDir string) (*v0manifest.ComponentManifest, error) {
	f, err := os.OpenFile(filepath.Join(srcDir, repository.ManifestFileName), os.O_RDONLY, os.ModePerm)
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer f.Close()

	m := &v0manifest.ComponentManifest{}
	if err := json.NewDecoder(f).Decode(m); err != nil {
		return nil, errors.Trace(err)
	}

	return m, nil
}

func readManifest1(dir, filename string, m *v1manifest.Manifest) error {
	f, err := os.Open(filepath.Join(dir, filename))
	if err != nil {
		return errors.Trace(err)
	}
	bytes, err := ioutil.ReadAll(f)
	if err != nil {
		return errors.Trace(err)
	}

	return json.Unmarshal(bytes, m)
}

func readVersions(srcDir, comp string) (*v0manifest.VersionManifest, error) {
	f, err := os.OpenFile(filepath.Join(srcDir, fmt.Sprintf("tiup-component-%s.index", comp)), os.O_RDONLY, os.ModePerm)
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer f.Close()

	m := &v0manifest.VersionManifest{}
	if err := json.NewDecoder(f).Decode(m); err != nil {
		return nil, errors.Trace(err)
	}

	return m, nil
}

func migrate(srcDir, dstDir string, rehash bool) error {
	m, err := readManifest0(srcDir)
	if err != nil {
		return errors.Trace(err)
	}

	if !rehash {
		if err := os.MkdirAll(filepath.Join(dstDir, "keys"), 0755); err != nil {
			return errors.Trace(err)
		}

		if err := os.MkdirAll(filepath.Join(dstDir, "manifests"), 0755); err != nil {
			return errors.Trace(err)
		}
	}

	initTime := time.Now().UTC()
	var (
		root  *v1manifest.Root
		index *v1manifest.Index
	)

	signedManifests := make(map[string]*v1manifest.Manifest)

	snapshot := v1manifest.NewSnapshot(initTime)

	if rehash { // read exist manifests
		s := &v1manifest.Manifest{
			Signed: &v1manifest.Snapshot{},
		}
		err = readManifest1(dstDir, filepath.Join("manifests", snapshot.Filename()), s)
		if err != nil {
			return errors.Trace(err)
		}
		snapshot = s.Signed.(*v1manifest.Snapshot)
		v1manifest.RenewManifest(snapshot, initTime)

		r := &v1manifest.Manifest{
			Signed: &v1manifest.Root{},
		}
		fmt.Println("reading root.json")
		if err := readManifest1(dstDir, filepath.Join("manifests", root.Filename()), r); err != nil {
			return errors.Trace(err)
		}
		root = r.Signed.(*v1manifest.Root)
		i := &v1manifest.Manifest{
			Signed: &v1manifest.Index{},
		}
		fmt.Println("reading index.json")
		fname := fmt.Sprintf("%d.%s",
			snapshot.Meta["/"+index.Filename()].Version, index.Filename())
		if err := readManifest1(dstDir, filepath.Join("manifests", fname), i); err != nil {
			return errors.Trace(err)
		}
		index = i.Signed.(*v1manifest.Index)
		index.Base().Version++
		v1manifest.RenewManifest(index, initTime)
	} else {
		root = v1manifest.NewRoot(initTime)
		index = v1manifest.NewIndex(initTime)
	}

	manifests := map[string]v1manifest.ValidManifest{
		v1manifest.ManifestTypeRoot:  root,
		v1manifest.ManifestTypeIndex: index,
	}

	// TODO: bootstrap a server instead of generating key
	keyDir := filepath.Join(dstDir, "keys")
	keys := map[string][]*v1manifest.KeyInfo{}

	tys := []string{ // root.json is not signed by default
		v1manifest.ManifestTypeIndex,
		v1manifest.ManifestTypeSnapshot,
		v1manifest.ManifestTypeTimestamp,
	}
	if rehash { // read keys
		for _, ty := range tys { // read manifest keys
			privKi := make([]*v1manifest.KeyInfo, 0)
			for id := range root.Roles[ty].Keys {
				pk := &v1manifest.KeyInfo{}
				f, err := os.Open(filepath.Join(dstDir, "keys",
					fmt.Sprintf("%s-%s.json", id[:v1manifest.ShortKeyIDLength], ty)))
				if err != nil {
					return errors.Trace(err)
				}
				bytes, err := ioutil.ReadAll(f)
				if err != nil {
					return errors.Trace(err)
				}
				if err = json.Unmarshal(bytes, pk); err != nil {
					return errors.Trace(err)
				}
				privKi = append(privKi, pk)
			}
			keys[ty] = privKi
		}
	} else { // generate new keys
		if signedRoot {
			tys = append(tys, v1manifest.ManifestTypeRoot)
		}

		for _, ty := range tys {
			if err := v1manifest.GenAndSaveKeys(keys, ty, int(v1manifest.ManifestsConfig[ty].Threshold), keyDir); err != nil {
				return errors.Trace(err)
			}
		}
	}

	// initial manifests
	genkey := func() (string, *v1manifest.KeyInfo, error) {
		priv, err := v1manifest.GenKeyInfo()
		if err != nil {
			return "", nil, err
		}

		id, err := priv.ID()
		if err != nil {
			return "", nil, err
		}

		return id, priv, nil
	}

	var (
		ownerkeyID   string
		ownerkeyInfo = &v1manifest.KeyInfo{}
	)
	if rehash {
		// read owner key (not considering index version, migrate assumes there
		// is no owner change between new and rehash
		for id := range index.Owners["pingcap"].Keys {
			ownerkeyID = id
			f, err := os.Open(filepath.Join(dstDir, "keys",
				fmt.Sprintf("%s-pingcap.json", ownerkeyID[:v1manifest.ShortKeyIDLength])))
			if err != nil {
				return errors.Trace(err)
			}
			bytes, err := ioutil.ReadAll(f)
			if err != nil {
				return errors.Trace(err)
			}
			if err = json.Unmarshal(bytes, ownerkeyInfo); err != nil {
				return errors.Trace(err)
			}
		}
	} else {
		// Initialize the index manifest
		ownerkeyID, ownerkeyInfo, err = genkey()
		if err != nil {
			return errors.Trace(err)
		}
		// save owner key
		if err := v1manifest.SaveKeyInfo(ownerkeyInfo, "pingcap", keyDir); err != nil {
			return errors.Trace(err)
		}

		ownerkeyPub, err := ownerkeyInfo.Public()
		if err != nil {
			return errors.Trace(err)
		}
		index.Owners["pingcap"] = v1manifest.Owner{
			Name: "PingCAP",
			Keys: map[string]*v1manifest.KeyInfo{
				ownerkeyID: ownerkeyPub,
			},
			Threshold: 1,
		}
	}

	hasTiup := false
	for _, comp := range m.Components {
		if comp.Name == "tiup" {
			hasTiup = true
			break
		}
	}

	// If TiUP it self is not in the v0 repo, build a dummy one from the default tarball
	tiupDesc := "Components manager for TiDB ecosystem"
	tiupVersions := &v0manifest.VersionManifest{
		Description: tiupDesc,
		Versions: []v0manifest.VersionInfo{
			{
				Version: v0manifest.Version(tiupver.NewTiUPVersion().SemVer()),
				Date:    time.Now().Format(time.RFC3339),
				Entry:   "tiup",
			},
		},
	}
	for _, goos := range []string{"linux", "darwin"} {
		for _, goarch := range []string{"amd64", "arm64"} {
			name := fmt.Sprintf("tiup-%s-%s.tar.gz", goos, goarch)
			path := filepath.Join(srcDir, name)
			if utils.IsExist(path) {
				tiupVersions.Versions[0].Platforms = append(tiupVersions.Versions[0].Platforms, repository.PlatformString(goos, goarch))
			}
		}
	}

	if !hasTiup {
		// Treat TiUP as a component
		tiup := v0manifest.ComponentInfo{
			Name:       "tiup",
			Desc:       tiupDesc,
			Standalone: false,
			Hide:       true,
		}

		m.Components = append(m.Components, tiup)
	}

	// Initialize the components manifest
	for _, comp := range m.Components {
		fmt.Println("found component", comp.Name)
		var versions *v0manifest.VersionManifest
		var err error
		versions, err = readVersions(srcDir, comp.Name)
		if comp.Name == "tiup" && os.IsNotExist(err) {
			versions = tiupVersions
		} else if err != nil {
			return errors.Trace(err)
		}

		platforms := map[string]map[string]v1manifest.VersionItem{}
		for _, v := range versions.Versions {
			for _, p := range v.Platforms {
				newp := p
				vs, found := platforms[newp]
				if !found {
					vs = map[string]v1manifest.VersionItem{}
					platforms[newp] = vs
				}

				filename := fmt.Sprintf("/%s-%s-%s.tar.gz", comp.Name, v.Version, strings.Join(
					strings.Split(newp, "/"),
					"-"))
				if comp.Name == "tiup" && utils.IsNotExist(filepath.Join(srcDir, filename)) {
					// if not versioned tiup package exist, use the unversioned one
					// for v0 manifest to generate a dummy component
					filename = fmt.Sprintf("/%s-%s.tar.gz", comp.Name, strings.Join(
						strings.Split(newp, "/"),
						"-"))
				}

				fmt.Printf("rehashing %s...\n", filepath.Join(srcDir, filename))
				hashes, length, err := ru.HashFile(filepath.Join(srcDir, filename))
				if err != nil {
					return errors.Trace(err)
				}
				// due to the nature of our CDN, all files are under the same URI base
				vs[v.Version.String()] = v1manifest.VersionItem{
					Yanked:   false,
					URL:      filename,
					Entry:    v.Entry,
					Released: v.Date,
					FileHash: v1manifest.FileHash{
						Hashes: hashes,
						Length: uint(length),
					},
				}
			}
		}

		// add nightly version, the versions for daily builds are all "nightly"
		// without date in v0 manifest, we keep that during migrate, but should
		// use new format with date number in version string when adding new
		// in the future
		var nightlyVer string
		if versions.Nightly != nil {
			v := versions.Nightly
			nightlyVer = v.Version.String()
			for _, p := range v.Platforms {
				newp := p
				vs, found := platforms[newp]
				if !found {
					vs = map[string]v1manifest.VersionItem{}
					platforms[newp] = vs
				}

				filename := fmt.Sprintf("/%s-%s-%s.tar.gz", comp.Name, tiupver.NightlyVersion,
					strings.Join(strings.Split(newp, "/"), "-"))
				hashes, length, err := ru.HashFile(path.Join(srcDir, filename))
				if err != nil {
					return errors.Trace(err)
				}
				// due to the nature of our CDN, all files are under the same URI base
				vs[v.Version.String()] = v1manifest.VersionItem{
					Yanked:   false,
					URL:      filename,
					Entry:    v.Entry,
					Released: v.Date,
					FileHash: v1manifest.FileHash{
						Hashes: hashes,
						Length: uint(length),
					},
				}
			}
		}

		component := &v1manifest.Component{
			SignedBase: v1manifest.SignedBase{
				Ty:          v1manifest.ManifestTypeComponent,
				SpecVersion: v1manifest.CurrentSpecVersion,
				Expires:     initTime.Add(v1manifest.ManifestsConfig[v1manifest.ManifestTypeComponent].Expire).Format(time.RFC3339),
				Version:     1, // initial repo starts with version 1
			},
			ID:          comp.Name,
			Description: comp.Desc,
			Nightly:     nightlyVer,
			Platforms:   platforms,
		}

		name := fmt.Sprintf("%s.json", component.ID)

		// update manifest version for component
		if rehash {
			component.Version = snapshot.Meta["/"+name].Version + 1
		}

		signedManifests[component.ID], err = v1manifest.SignManifest(component, ownerkeyInfo)
		if err != nil {
			return errors.Trace(err)
		}

		index.Components[comp.Name] = v1manifest.ComponentItem{
			Yanked:     false,
			Owner:      "pingcap",
			URL:        fmt.Sprintf("/%s", name),
			Standalone: comp.Standalone,
			Hidden:     comp.Hide,
		}
	}

	// sign index and snapshot
	signedManifests[v1manifest.ManifestTypeIndex], err = v1manifest.SignManifest(index, keys[v1manifest.ManifestTypeIndex]...)
	if err != nil {
		return errors.Trace(err)
	}

	// snapshot and timestamp are the last two manifests to be initialized

	// Initialize timestamp
	timestamp := v1manifest.NewTimestamp(initTime)
	if rehash {
		t := &v1manifest.Manifest{
			Signed: &v1manifest.Timestamp{},
		}
		if err := readManifest1(dstDir, filepath.Join("manifests", timestamp.Filename()), t); err != nil {
			return errors.Trace(err)
		}
		timestamp.Version = t.Signed.(*v1manifest.Timestamp).Version
	}

	manifests[v1manifest.ManifestTypeTimestamp] = timestamp
	manifests[v1manifest.ManifestTypeSnapshot] = snapshot

	if !rehash {
		// Initialize the root manifest
		for _, m := range manifests {
			if err := root.SetRole(m, keys[m.Base().Ty]...); err != nil {
				return errors.Trace(err)
			}
		}

		if signedRoot {
			signedManifests[v1manifest.ManifestTypeRoot], err = v1manifest.SignManifest(root, keys[v1manifest.ManifestTypeRoot]...)
			if err != nil {
				return errors.Trace(err)
			}
		} else {
			// root manifest is not signed, and need to be manually signed by root key owners
			signedManifests[v1manifest.ManifestTypeRoot] = &v1manifest.Manifest{
				Signed: root,
			}
		}
	}

	// populate snapshot
	snapshot, err = snapshot.SetVersions(signedManifests)
	if err != nil {
		return errors.Trace(err)
	}
	signedManifests[v1manifest.ManifestTypeSnapshot], err = v1manifest.SignManifest(snapshot, keys[v1manifest.ManifestTypeSnapshot]...)
	if err != nil {
		return errors.Trace(err)
	}

	// populate timestamp
	timestamp, err = timestamp.SetSnapshot(signedManifests[v1manifest.ManifestTypeSnapshot])
	if err != nil {
		return errors.Trace(err)
	}
	if rehash {
		timestamp.Version++
	}

	signedManifests[v1manifest.ManifestTypeTimestamp], err = v1manifest.SignManifest(timestamp, keys[v1manifest.ManifestTypeTimestamp]...)
	if err != nil {
		return errors.Trace(err)
	}
	for _, m := range signedManifests {
		fname := filepath.Join(dstDir, "manifests", m.Signed.Filename())
		switch m.Signed.Base().Ty {
		case v1manifest.ManifestTypeRoot:
			err := v1manifest.WriteManifestFile(repository.FnameWithVersion(fname, m.Signed.Base().Version), m)
			if err != nil {
				return errors.Trace(err)
			}
			// A copy of the newest version which is 1.
			err = v1manifest.WriteManifestFile(fname, m)
			if err != nil {
				return errors.Trace(err)
			}
		case v1manifest.ManifestTypeComponent, v1manifest.ManifestTypeIndex:
			err := v1manifest.WriteManifestFile(repository.FnameWithVersion(fname, m.Signed.Base().Version), m)
			if err != nil {
				return errors.Trace(err)
			}
		default:
			err = v1manifest.WriteManifestFile(fname, m)
			if err != nil {
				return errors.Trace(err)
			}
		}
	}
	return nil
}

func update(dir string, keyFiles []string, isPublicKey bool) error {
	manifests := make(map[string]*v1manifest.Manifest)
	keys := make(map[string][]*v1manifest.KeyInfo)
	currTime := time.Now().UTC()
	var err error

	// read root.json
	root := v1manifest.Manifest{
		Signed: &v1manifest.Root{},
	}
	if err = readManifest1(filepath.Join(dir, "manifests"), v1manifest.ManifestFilenameRoot, &root); err != nil {
		return errors.Trace(err)
	}
	manifests[v1manifest.ManifestTypeRoot] = &root

	// if inputs are public keys, just add them to the root's key list and return
	if isPublicKey {
		for _, fname := range keyFiles {
			var kp v1manifest.KeyInfo
			f, err := os.Open(fname)
			if err != nil {
				return errors.Trace(err)
			}
			bytes, err := ioutil.ReadAll(f)
			if err != nil {
				return errors.Trace(err)
			}

			if err := json.Unmarshal(bytes, &kp); err != nil {
				return errors.Trace(err)
			}

			if err := root.Signed.(*v1manifest.Root).AddKey(v1manifest.ManifestTypeRoot, &kp); err != nil {
				return errors.Trace(err)
			}
		}
		// write the file
		fname := filepath.Join(dir, "manifests", root.Signed.Filename())
		err := v1manifest.WriteManifestFile(repository.FnameWithVersion(fname, root.Signed.Base().Version), &root)
		if err != nil {
			return errors.Trace(err)
		}
		// A copy of the newest version which is 1.
		err = v1manifest.WriteManifestFile(fname, &root)
		if err != nil {
			return errors.Trace(err)
		}
		return nil
	}

	// append signatures
	for _, fname := range keyFiles { // signed root files
		sr := v1manifest.Manifest{
			Signed: &v1manifest.Root{},
		}
		if err = readManifest1("", fname, &sr); err != nil {
			return errors.Trace(err)
		}
		root.AddSignature(sr.Signatures)
	}

	// read snapshot.json
	snapshotOld := v1manifest.Manifest{
		Signed: &v1manifest.Snapshot{},
	}
	if err := readManifest1(filepath.Join(dir, "manifests"), v1manifest.ManifestFilenameSnapshot, &snapshotOld); err != nil {
		return errors.Trace(err)
	}
	for _, sig := range snapshotOld.Signatures {
		id := sig.KeyID[:v1manifest.ShortKeyIDLength]
		f, err := os.Open(filepath.Join(dir, "keys", fmt.Sprintf(
			"%s-%s", id, v1manifest.ManifestFilenameSnapshot,
		)))
		if err != nil {
			return errors.Trace(err)
		}
		defer f.Close()

		ki := v1manifest.KeyInfo{}
		if err := json.NewDecoder(f).Decode(&ki); err != nil {
			return errors.Trace(err)
		}
		if keys[v1manifest.ManifestTypeSnapshot] == nil {
			keys[v1manifest.ManifestTypeSnapshot] = make([]*v1manifest.KeyInfo, 0)
		}
		keys[v1manifest.ManifestTypeSnapshot] = append(keys[v1manifest.ManifestTypeSnapshot], &ki)
	}

	// update snapshot
	snapshot, err := snapshotOld.Signed.(*v1manifest.Snapshot).SetVersions(manifests)
	if err != nil {
		return errors.Trace(err)
	}
	manifests[v1manifest.ManifestTypeSnapshot], err = v1manifest.SignManifest(snapshot, keys[v1manifest.ManifestTypeSnapshot]...)
	if err != nil {
		return errors.Trace(err)
	}

	// read timestamp.json
	tsOld := v1manifest.Manifest{
		Signed: &v1manifest.Timestamp{},
	}
	if err := readManifest1(filepath.Join(dir, "manifests"), v1manifest.ManifestFilenameTimestamp, &tsOld); err != nil {
		return errors.Trace(err)
	}
	manifests[v1manifest.ManifestTypeTimestamp] = &tsOld
	for _, sig := range manifests[v1manifest.ManifestTypeTimestamp].Signatures {
		id := sig.KeyID[:v1manifest.ShortKeyIDLength]
		f, err := os.Open(filepath.Join(dir, "keys", fmt.Sprintf(
			"%s-%s", id, v1manifest.ManifestFilenameTimestamp,
		)))
		if err != nil {
			return errors.Trace(err)
		}
		defer f.Close()

		ki := v1manifest.KeyInfo{}
		if err := json.NewDecoder(f).Decode(&ki); err != nil {
			return errors.Trace(err)
		}
		if keys[v1manifest.ManifestTypeTimestamp] == nil {
			keys[v1manifest.ManifestTypeTimestamp] = make([]*v1manifest.KeyInfo, 0)
		}
		keys[v1manifest.ManifestTypeTimestamp] = append(keys[v1manifest.ManifestTypeTimestamp], &ki)
	}

	// update timestamp
	timestamp, err := v1manifest.NewTimestamp(currTime).SetSnapshot(manifests[v1manifest.ManifestTypeSnapshot])
	if err != nil {
		return errors.Trace(err)
	}
	timestamp.Base().Version = manifests[v1manifest.ManifestTypeTimestamp].Signed.Base().Version + 1
	manifests[v1manifest.ManifestTypeTimestamp], err = v1manifest.SignManifest(timestamp, keys[v1manifest.ManifestTypeTimestamp]...)
	if err != nil {
		return errors.Trace(err)
	}

	for _, m := range manifests {
		fname := filepath.Join(dir, "manifests", m.Signed.Filename())
		switch m.Signed.Base().Ty {
		case v1manifest.ManifestTypeRoot:
			err := v1manifest.WriteManifestFile(repository.FnameWithVersion(fname, m.Signed.Base().Version), m)
			if err != nil {
				return errors.Trace(err)
			}
			// A copy of the newest version which is 1.
			err = v1manifest.WriteManifestFile(fname, m)
			if err != nil {
				return errors.Trace(err)
			}
		default:
			err = v1manifest.WriteManifestFile(fname, m)
			if err != nil {
				return errors.Trace(err)
			}
		}
	}

	return nil
}
