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
	"os"
	"path/filepath"
	"strings"
	"time"

	cjson "github.com/gibson042/canonicaljson-go"
	"github.com/pingcap-incubator/tiup/pkg/repository"
	"github.com/pingcap-incubator/tiup/pkg/repository/v0manifest"
	"github.com/pingcap-incubator/tiup/pkg/repository/v1manifest"
	"github.com/pingcap/errors"
	"github.com/spf13/cobra"
)

func main() {
	cmd := &cobra.Command{
		Use:           "migrate <src-dir> <dst-dir>",
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

	if err := cmd.Execute(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}

func readManifest(srcDir string) (*v0manifest.ComponentManifest, error) {
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

func migrate(srcDir, dstDir string) error {
	m, err := readManifest(srcDir)
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

	index.Owners["pingcap"] = v1manifest.Owner{
		Name: "PingCAP",
		Keys: map[string]*v1manifest.KeyInfo{
			ownerkeyID: ownerkeyInfo,
		},
	}
	signedManifests[v1manifest.ManifestTypeIndex], err = v1manifest.SignManifest(index, ownerkeyInfo)
	if err != nil {
		return err
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

		name := fmt.Sprintf("%s-pingcap.json", ownerkeyID[:16])
		writer, err := os.OpenFile(filepath.Join(dstDir, "keys", name), os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
		if err != nil {
			return err
		}
		defer writer.Close()
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
		stat, err := writer.Stat()
		if err != nil {
			return errors.Trace(err)
		}

		snapshot.Meta[name] = v1manifest.FileVersion{Version: 1, Length: uint(stat.Size())}
	}

	// sign index and snapshot
	signedManifests[v1manifest.ManifestTypeIndex], err = v1manifest.SignManifest(index, ownerkeyInfo)
	if err != nil {
		return err
	}

	// snapshot and timestamp are the last two manifests to be initialized
	// init snapshot
	snapshot, err = snapshot.SetVersions(signedManifests)
	if err != nil {
		return err
	}
	signedManifests[v1manifest.ManifestTypeSnapshot], err = v1manifest.SignManifest(snapshot, ownerkeyInfo)
	if err != nil {
		return err
	}

	// Initialize timestamp
	timestamp := v1manifest.NewTimestamp(initTime)
	manifests[v1manifest.ManifestTypeTimestamp] = timestamp

	// Initialize the root manifest
	for _, m := range manifests {
		root.SetRole(m)
	}

	signedManifests[v1manifest.ManifestTypeRoot], err = v1manifest.SignManifest(root, ownerkeyInfo)
	if err != nil {
		return err
	}

	hash, n, err := hashManifest(signedManifests[v1manifest.ManifestTypeSnapshot])
	if err != nil {
		return errors.Trace(err)
	}
	timestamp.Meta = map[string]v1manifest.FileHash{
		v1manifest.ManifestFilenameSnapshot: {
			Hashes: hash,
			Length: n,
		},
	}
	signedManifests[v1manifest.ManifestTypeTimestamp], err = v1manifest.SignManifest(timestamp, ownerkeyInfo)
	if err != nil {
		return err
	}
	for _, m := range signedManifests {
		writer, err := os.OpenFile(filepath.Join(dstDir, "manifests", m.Signed.Filename()), os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
		if err != nil {
			return err
		}
		defer writer.Close()

		if err = v1manifest.WriteManifest(writer, m); err != nil {
			return err
		}
	}
	return nil
}
