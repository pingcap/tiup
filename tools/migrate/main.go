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
	"strings"
	"time"

	cjson "github.com/gibson042/canonicaljson-go"
	"github.com/pingcap-incubator/tiup/pkg/repository"
	"github.com/pingcap-incubator/tiup/pkg/repository/crypto"
	"github.com/pingcap-incubator/tiup/pkg/repository/manifest"
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

func readManifest(srcDir string) (*manifest.ComponentManifest, error) {
	f, err := os.OpenFile(filepath.Join(srcDir, repository.ManifestFileName), os.O_RDONLY, os.ModePerm)
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer f.Close()

	m := &manifest.ComponentManifest{}
	if err := json.NewDecoder(f).Decode(m); err != nil {
		return nil, errors.Trace(err)
	}

	return m, nil
}

func readVersions(srcDir, comp string) (*manifest.VersionManifest, error) {
	f, err := os.OpenFile(filepath.Join(srcDir, fmt.Sprintf("tiup-component-%s.index", comp)), os.O_RDONLY, os.ModePerm)
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer f.Close()

	m := &manifest.VersionManifest{}
	if err := json.NewDecoder(f).Decode(m); err != nil {
		return nil, errors.Trace(err)
	}

	return m, nil
}

func hashes(srcDir, filename string) (map[string]string, int64, error) {
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
		"sha256": hex.EncodeToString(s256.Sum(nil)),
		"sha512": hex.EncodeToString(s512.Sum(nil)),
	}
	return hashes, n, err
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
		root     = manifest.NewRoot(initTime)
		index    = manifest.NewIndex(initTime)
	)

	// initial manifests
	manifests := map[string]manifest.ValidManifest{
		manifest.ManifestTypeRoot:  root,
		manifest.ManifestTypeIndex: index,
	}

	// snapshot and timestamp are the last two manifests to be initialized
	// init snapshot
	snapshot := manifest.NewSnapshot(initTime).SetVersions(manifests)
	manifests[manifest.ManifestTypeSnapshot] = snapshot

	privKeys := map[string]*crypto.RSAPrivKey{}
	keyNames := map[string]string{}

	genkey := func(name string) (string, *manifest.KeyInfo, error) {
		// Generate RSA pairs
		pub, priv, err := crypto.RsaPair()
		if err != nil {
			return "", nil, errors.Trace(err)
		}

		pubSer, err := pub.Serialize()
		if err != nil {
			return "", nil, errors.Trace(err)
		}

		privSer, err := priv.Serialize()
		if err != nil {
			return "", nil, errors.Trace(err)
		}
		err = ioutil.WriteFile(filepath.Join(dstDir, "keys", name), privSer, os.ModePerm)
		if err != nil {
			return "", nil, errors.Trace(err)
		}

		keyInfo := &manifest.KeyInfo{
			Algorithms: []string{"sha256"},
			Type:       "rsa",
			Value: map[string]string{
				"public": string(pubSer),
			},
			Scheme: "rsassa-pss-sha256",
		}

		key, err := cjson.Marshal(keyInfo)
		if err != nil {
			return "", nil, errors.Trace(err)
		}

		hash := sha256.Sum256(key)
		keyId := hex.EncodeToString(hash[:])

		privKeys[name] = priv
		keyNames[name] = keyId

		return keyId, keyInfo, nil
	}

	// Initialize the index manifest
	keyId, keyInfo, err := genkey("pingcap")
	if err != nil {
		return errors.Trace(err)
	}
	index.Owners["pingcap"] = manifest.Owner{
		Name: "PingCAP",
		Keys: map[string]*manifest.KeyInfo{
			keyId: keyInfo,
		},
	}

	// Initialize the components manifest
	for _, comp := range m.Components {
		fmt.Println("found component", comp.Name)
		versions, err := readVersions(srcDir, comp.Name)
		if err != nil {
			return err
		}

		platforms := map[string]map[string]manifest.VersionItem{}
		for _, v := range versions.Versions {
			for _, p := range v.Platforms {
				newp := strings.Replace(p, "/", "-", -1)
				vs, found := platforms[newp]
				if !found {
					vs = map[string]manifest.VersionItem{}
					platforms[newp] = vs
				}

				filename := fmt.Sprintf("/%s-%s-%s.tar.gz", comp.Name, v.Version, newp)
				hashes, length, err := hashes(srcDir, filename)
				if err != nil {
					return err
				}
				vs[v.Version.String()] = manifest.VersionItem{
					Yanked:   false,
					URL:      filename,
					Entry:    v.Entry,
					Released: v.Date,
					Hashes:   hashes,
					Length:   length,
				}
			}
		}

		component := &manifest.Component{
			SignedBase: manifest.SignedBase{
				Ty:          manifest.ManifestTypeComponent,
				SpecVersion: manifest.CurrentSpecVersion,
				Expires:     initTime.Add(manifest.ManifestsConfig[manifest.ManifestTypeComponent].Expire).Format(time.RFC3339),
				Version:     1, // initial repo starts with version 1
			},
			Name:        comp.Name,
			Description: comp.Desc,
			Platforms:   platforms,
		}

		name := fmt.Sprintf("%s.json", comp.Name)
		writer, err := os.OpenFile(filepath.Join(dstDir, "manifests", name), os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
		if err != nil {
			return err
		}
		defer writer.Close()
		if err = manifest.SignAndWrite(writer, component, keyId, privKeys["pingcap"]); err != nil {
			return err
		}

		stat, err := writer.Stat()
		if err != nil {
			return errors.Trace(err)
		}

		index.Components[comp.Name] = manifest.ComponentItem{
			Name:        comp.Name,
			Description: comp.Desc,
			Yanked:      false,
			Owner:       "pingcap",
			URL:         fmt.Sprintf("/%s", name),
			Length:      stat.Size(),
			Threshold:   0,
		}

		snapshot.Meta[name] = manifest.FileVersion{Version: 1}
	}

	// Initialize timestamp
	timestamp := manifest.NewTimestamp(initTime)
	manifests[manifest.ManifestTypeTimestamp] = timestamp

	// Initialize the root manifest
	for _, m := range manifests {
		root.SetRole(m)
		keyId, keyInfo, err := genkey(m.Base().Ty)
		if err != nil {
			return errors.Trace(err)
		}
		root.Roles[m.Base().Ty].Keys[keyId] = keyInfo
	}

	for ty, m := range manifests {
		if ty == manifest.ManifestTypeTimestamp {
			filename := manifest.ManifestTypeSnapshot + ".json"
			hash, n, err := hashes(filepath.Join(dstDir, "manifests"), filename)
			if err != nil {
				return errors.Trace(err)
			}
			timestamp.Meta = map[string]manifest.FileHash{
				filename: {
					Hashes: hash,
					Length: uint(n),
				},
			}
		}
		writer, err := os.OpenFile(filepath.Join(dstDir, "manifests", m.Filename()), os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
		if err != nil {
			return err
		}
		defer writer.Close()
		// TODO: support multiples keys
		keyId := keyNames[m.Base().Ty]
		if err = manifest.SignAndWrite(writer, m, keyId, privKeys[m.Base().Ty]); err != nil {
			return err
		}
	}
	return nil
}
