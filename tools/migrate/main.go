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
	"os"
	"path/filepath"
	"time"

	"github.com/pingcap-incubator/tiup/pkg/repository"
	"github.com/pingcap/errors"
	"github.com/spf13/cobra"
)

func main() {
	cmd := &cobra.Command{
		Use:           "migrate <src-dir> <dst-dir>",
		Short:         "Generate new manifest base on exists ola repository",
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

func readManifest(srcDir string) (*repository.ComponentManifest, error) {
	f, err := os.OpenFile(filepath.Join(srcDir, repository.ManifestFileName), os.O_RDONLY, os.ModePerm)
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer f.Close()

	m := &repository.ComponentManifest{}
	if err := json.NewDecoder(f).Decode(m); err != nil {
		return nil, errors.Trace(err)
	}

	return m, nil
}

func readVersions(srcDir, comp string) (*repository.VersionManifest, error) {
	f, err := os.OpenFile(filepath.Join(srcDir, fmt.Sprintf("tiup-component-%s.index", comp)), os.O_RDONLY, os.ModePerm)
	if err != nil {
		return nil, errors.Trace(err)
	}
	defer f.Close()

	m := &repository.VersionManifest{}
	if err := json.NewDecoder(f).Decode(m); err != nil {
		return nil, errors.Trace(err)
	}

	return m, nil
}

func migrate(srcDir, dstDir string) error {
	manifest, err := readManifest(srcDir)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return errors.Trace(err)
	}

	var (
		initTime = time.Now()
		root     = repository.NewRoot(initTime)
		index    = repository.NewIndex(initTime)
	)

	// initial manifests
	manifests := map[string]repository.ValidManifest{
		repository.ManifestTypeRoot:  root,
		repository.ManifestTypeIndex: index,
	}

	// snapshot and timestamp are the last two manifests to be initialized
	// init snapshot
	snapshot := repository.NewSnapshot(initTime).SetVersions(manifests)
	// init timestamp
	timestamp, err := repository.NewTimestamp(initTime).SetSnapshot(manifests[repository.ManifestTypeSnapshot].(*repository.Snapshot))
	if err != nil {
		return err
	}

	manifests[repository.ManifestTypeSnapshot] = snapshot
	manifests[repository.ManifestTypeTimestamp] = timestamp

	for _, m := range manifests {
		root.SetRole(m)
	}

	for _, comp := range manifest.Components {
		fmt.Println("found component", comp.Name)
		versions, err := readVersions(srcDir, comp.Name)
		if err != nil {
			return err
		}
	}

	return nil
}
