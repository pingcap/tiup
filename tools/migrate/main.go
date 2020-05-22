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
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	cjson "github.com/gibson042/canonicaljson-go"
	"github.com/pingcap-incubator/tiup/pkg/repository"
	"github.com/pingcap-incubator/tiup/pkg/repository/v0manifest"
	"github.com/pingcap-incubator/tiup/pkg/repository/v1manifest"
	"github.com/pingcap/errors"
	"github.com/spf13/cobra"
)

// for simplify just set the length of manifest file a value lager than the true value
var limitLength uint = 1024 * 1024

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

			return migrate(args[0], args[1])
		},
	}

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
			return update(args[0], args[1:])
		},
	}

	cmd.AddCommand(
		newCmd,
		updateCmd,
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
		return err
	}
	bytes, err := ioutil.ReadAll(f)
	if err != nil {
		return err
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

func hashFile(srcDir, filename string) (map[string]string, int64, error) {
	path := filepath.Join(srcDir, filename)
	s256 := sha256.New()
	s512 := sha512.New()
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer file.Close()
	n, err := io.Copy(io.MultiWriter(s256, s512), file)

	hashes := map[string]string{
		v1manifest.SHA256: hex.EncodeToString(s256.Sum(nil)),
		v1manifest.SHA512: hex.EncodeToString(s512.Sum(nil)),
	}
	return hashes, n, err
}

func hashManifest(m *v1manifest.Manifest) (map[string]string, uint, error) {
	bytes, err := cjson.Marshal(m)
	if err != nil {
		return nil, 0, err
	}

	s256 := sha256.Sum256(bytes)
	s512 := sha512.Sum512(bytes)

	return map[string]string{
		v1manifest.SHA256: hex.EncodeToString(s256[:]),
		v1manifest.SHA512: hex.EncodeToString(s512[:]),
	}, uint(len(bytes)), nil
}

func fnameWithVersion(fname string, version uint) string {
	base := filepath.Base(fname)
	dir := filepath.Dir(fname)

	versionBase := strconv.Itoa(int(version)) + "." + base
	return filepath.Join(dir, versionBase)
}

func migrate(srcDir, dstDir string) error {
	m, err := readManifest0(srcDir)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Join(dstDir, "keys"), 0755); err != nil {
		return errors.Trace(err)
	}

	if err := os.MkdirAll(filepath.Join(dstDir, "manifests"), 0755); err != nil {
		return errors.Trace(err)
	}

	var (
		initTime = time.Now()
		root     = v1manifest.NewRoot(initTime)
		index    = v1manifest.NewIndex(initTime)
	)

	// TODO: bootstrap a server instead of generating key
	keyDir := filepath.Join(dstDir, "keys")
	keys := map[string][]*v1manifest.KeyInfo{}

	tys := []string{ // root.json is not signed
		v1manifest.ManifestTypeIndex,
		v1manifest.ManifestTypeSnapshot,
		v1manifest.ManifestTypeTimestamp,
	}
	for _, ty := range tys {
		if err := v1manifest.GenAndSaveKeys(keys, ty, int(v1manifest.ManifestsConfig[ty].Threshold), keyDir); err != nil {
			return err
		}
	}

	// initial manifests
	manifests := map[string]v1manifest.ValidManifest{
		v1manifest.ManifestTypeRoot:  root,
		v1manifest.ManifestTypeIndex: index,
	}
	signedManifests := make(map[string]*v1manifest.Manifest)

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

	// Initialize the index manifest
	ownerkeyID, ownerkeyInfo, err := genkey()
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
	}

	snapshot := v1manifest.NewSnapshot(initTime)

	// Initialize the components manifest
	for _, comp := range m.Components {
		fmt.Println("found component", comp.Name)
		versions, err := readVersions(srcDir, comp.Name)
		if err != nil {
			return err
		}

		platforms := map[string]map[string]v1manifest.VersionItem{}
		for _, v := range versions.Versions {
			for _, p := range v.Platforms {
				newp := strings.Replace(p, "/", "-", -1)
				vs, found := platforms[newp]
				if !found {
					vs = map[string]v1manifest.VersionItem{}
					platforms[newp] = vs
				}

				filename := fmt.Sprintf("/%s-%s-%s.tar.gz", comp.Name, v.Version, newp)
				hashes, length, err := hashFile(srcDir, filename)
				if err != nil {
					return err
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
			Name:        comp.Name,
			Description: comp.Desc,
			Platforms:   platforms,
		}

		name := fmt.Sprintf("%s.json", component.ID)
		signedManifests[component.ID], err = v1manifest.SignManifest(component, ownerkeyInfo)
		if err != nil {
			return err
		}

		index.Components[comp.Name] = v1manifest.ComponentItem{
			Yanked:    false,
			Owner:     "pingcap",
			URL:       fmt.Sprintf("/%s", name),
			Threshold: 1,
		}

		bytes, err := cjson.Marshal(signedManifests[component.ID])
		if err != nil {
			return err
		}
		var _ = len(bytes) // this length is the not final length, since we still change the manifests before write it to disk.
		snapshot.Meta["/"+name] = v1manifest.FileVersion{
			Version: 1,
			Length:  limitLength,
		}
	}

	// sign index and snapshot
	signedManifests[v1manifest.ManifestTypeIndex], err = v1manifest.SignManifest(index, keys[v1manifest.ManifestTypeIndex]...)
	if err != nil {
		return err
	}

	// snapshot and timestamp are the last two manifests to be initialized

	// Initialize timestamp
	timestamp := v1manifest.NewTimestamp(initTime)
	manifests[v1manifest.ManifestTypeTimestamp] = timestamp
	manifests[v1manifest.ManifestTypeSnapshot] = snapshot

	// Initialize the root manifest
	for _, m := range manifests {
		root.SetRole(m, keys[m.Base().Ty]...)
	}

	// root manifest is not signed, and need to be manually signed by root key owners
	signedManifests[v1manifest.ManifestTypeRoot] = &v1manifest.Manifest{
		Signed: root,
	}

	// init snapshot
	snapshot, err = snapshot.SetVersions(signedManifests)
	if err != nil {
		return err
	}
	signedManifests[v1manifest.ManifestTypeSnapshot], err = v1manifest.SignManifest(snapshot, keys[v1manifest.ManifestTypeSnapshot]...)
	if err != nil {
		return err
	}

	hash, _, err := hashManifest(signedManifests[v1manifest.ManifestTypeSnapshot])
	if err != nil {
		return errors.Trace(err)
	}
	timestamp.Meta = map[string]v1manifest.FileHash{
		v1manifest.ManifestURLSnapshot: {
			Hashes: hash,
			Length: limitLength,
		},
	}
	signedManifests[v1manifest.ManifestTypeTimestamp], err = v1manifest.SignManifest(timestamp, keys[v1manifest.ManifestTypeTimestamp]...)
	if err != nil {
		return err
	}
	for _, m := range signedManifests {
		fname := filepath.Join(dstDir, "manifests", m.Signed.Filename())
		switch m.Signed.Base().Ty {
		case v1manifest.ManifestTypeRoot:
			err := writeManifest(fnameWithVersion(fname, 1), m)
			if err != nil {
				return err
			}
			// A copy of the newest version which is 1.
			err = writeManifest(fname, m)
			if err != nil {
				return err
			}
		case v1manifest.ManifestTypeComponent, v1manifest.ManifestTypeIndex:
			err := writeManifest(fnameWithVersion(fname, 1), m)
			if err != nil {
				return err
			}
		default:
			err = writeManifest(fname, m)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func update(dir string, signedFiles []string) error {
	manifests := make(map[string]*v1manifest.Manifest)
	keys := make(map[string][]*v1manifest.KeyInfo)
	currTime := time.Now().UTC()
	var err error

	// read root.json
	root := v1manifest.Manifest{
		Signed: &v1manifest.Root{},
	}
	if err = readManifest1(filepath.Join(dir, "manifests"), v1manifest.ManifestFilenameRoot, &root); err != nil {
		return err
	}
	manifests[v1manifest.ManifestTypeRoot] = &root

	// append signatures
	for _, fname := range signedFiles {
		sr := v1manifest.Manifest{
			Signed: &v1manifest.Root{},
		}
		if err = readManifest1("", fname, &sr); err != nil {
			return err
		}
		root.AddSignature(sr.Signatures)
	}

	// read snapshot.json
	snapshotOld := v1manifest.Manifest{
		Signed: &v1manifest.Snapshot{},
	}
	if err := readManifest1(filepath.Join(dir, "manifests"), v1manifest.ManifestFilenameSnapshot, &snapshotOld); err != nil {
		return err
	}
	for _, sig := range snapshotOld.Signatures {
		id := sig.KeyID[:16]
		f, err := os.Open(filepath.Join(dir, "keys", fmt.Sprintf(
			"%s-%s", id, v1manifest.ManifestFilenameSnapshot,
		)))
		if err != nil {
			return err
		}
		defer f.Close()

		ki := v1manifest.KeyInfo{}
		if err := json.NewDecoder(f).Decode(&ki); err != nil {
			return err
		}
		if keys[v1manifest.ManifestTypeSnapshot] == nil {
			keys[v1manifest.ManifestTypeSnapshot] = make([]*v1manifest.KeyInfo, 0)
		}
		keys[v1manifest.ManifestTypeSnapshot] = append(keys[v1manifest.ManifestTypeSnapshot], &ki)
	}

	// update snapshot
	snapshot, err := snapshotOld.Signed.(*v1manifest.Snapshot).SetVersions(manifests)
	if err != nil {
		return err
	}
	manifests[v1manifest.ManifestTypeSnapshot], err = v1manifest.SignManifest(snapshot, keys[v1manifest.ManifestTypeSnapshot]...)
	if err != nil {
		return err
	}

	// read timestamp.json
	tsOld := v1manifest.Manifest{
		Signed: &v1manifest.Timestamp{},
	}
	if err := readManifest1(filepath.Join(dir, "manifests"), v1manifest.ManifestFilenameTimestamp, &tsOld); err != nil {
		return err
	}
	manifests[v1manifest.ManifestTypeTimestamp] = &tsOld
	for _, sig := range manifests[v1manifest.ManifestTypeTimestamp].Signatures {
		id := sig.KeyID[:16]
		f, err := os.Open(filepath.Join(dir, "keys", fmt.Sprintf(
			"%s-%s", id, v1manifest.ManifestFilenameTimestamp,
		)))
		if err != nil {
			return err
		}
		defer f.Close()

		ki := v1manifest.KeyInfo{}
		if err := json.NewDecoder(f).Decode(&ki); err != nil {
			return err
		}
		if keys[v1manifest.ManifestTypeTimestamp] == nil {
			keys[v1manifest.ManifestTypeTimestamp] = make([]*v1manifest.KeyInfo, 0)
		}
		keys[v1manifest.ManifestTypeTimestamp] = append(keys[v1manifest.ManifestTypeTimestamp], &ki)
	}

	// update timestamp
	timestamp, err := v1manifest.NewTimestamp(currTime).SetSnapshot(manifests[v1manifest.ManifestTypeSnapshot])
	if err != nil {
		return err
	}
	timestamp.Base().Version = manifests[v1manifest.ManifestTypeTimestamp].Signed.Base().Version + 1
	manifests[v1manifest.ManifestTypeTimestamp], err = v1manifest.SignManifest(timestamp, keys[v1manifest.ManifestTypeTimestamp]...)
	if err != nil {
		return err
	}

	for _, m := range manifests {
		fname := filepath.Join(dir, "manifests", m.Signed.Filename())
		switch m.Signed.Base().Ty {
		case v1manifest.ManifestTypeRoot:
			err := writeManifest(fnameWithVersion(fname, 1), m)
			if err != nil {
				return err
			}
			// A copy of the newest version which is 1.
			err = writeManifest(fname, m)
			if err != nil {
				return err
			}
		default:
			err = writeManifest(fname, m)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func writeManifest(fname string, m *v1manifest.Manifest) error {
	writer, err := os.OpenFile(fname, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer writer.Close()
	return v1manifest.WriteManifest(writer, m)
}
