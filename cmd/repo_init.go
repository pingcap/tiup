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

package cmd

import (
	"os"
	"time"

	"github.com/pingcap-incubator/tiup/pkg/meta"
	"github.com/pingcap-incubator/tiup/pkg/repository"
	"github.com/pingcap-incubator/tiup/pkg/utils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func newRepoInitCmd(env *meta.Environment) *cobra.Command {
	var (
		pubKey  string // public key of root
		privKey string // private key of root
	)
	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initialise an empty repository",
		Long: `Initialise an empty TiUP repository at given path. If path is not specified, the
current working directory (".") will be used.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				repoPath string
				err      error
			)
			if len(args) == 1 {
				repoPath = args[0]
			}

			// create the target path if not exist
			if utils.IsNotExist(repoPath) {
				if err = os.Mkdir(repoPath, 0755); err != nil {
					return err
				}
			}
			// init requires an empty path to use
			empty, err := utils.IsEmptyDir(repoPath)
			if err != nil {
				return err
			}
			if !empty {
				return errors.Errorf("the target path '%s' is not an empty directory", repoPath)
			}

			return initRepo(repoPath)
		},
	}

	cmd.Flags().StringVar(&pubKey, "pubkey", "", "Path to the public key file")
	cmd.Flags().StringVar(&privKey, "privkey", "", "Path to the private key file")

	return cmd
}

func initRepo(path string) error {
	currTime := time.Now()
	repoManifests := repository.NewManifests(path)
	// TODO: set key store

	// initial manifests
	newManifests := make([]*repository.Manifest, 0)

	// init the root manifest
	root := &repository.Root{
		SignedBase: repository.SignedBase{
			Ty:          "root",
			SpecVersion: "TODO",
			Expires:     currTime.UTC().Add(time.Hour * 24 * 365).Format(time.RFC3339), // 1y
			Version:     1,                                                             // initial repo starts with version 1
		},
		Roles: make(map[string]*repository.RoleMeta),
	}
	rootManifest := &repository.Manifest{
		Signed: root,
	}
	root.Roles[root.Filename()] = root.GetRole()
	newManifests = append(newManifests, rootManifest)

	// init index
	index := &repository.Index{
		SignedBase: repository.SignedBase{
			Ty:          "index",
			SpecVersion: "TODO",
			Expires:     currTime.UTC().Add(time.Hour * 24 * 365).Format(time.RFC3339), // 1y
			Version:     1,
		},
		Owners:            make(map[string]repository.Owner),
		Components:        make(map[string]repository.Component),
		DefaultComponents: make([]string, 0),
	}
	// TODO add initial owner to index.Owners
	root.Roles[index.Filename()] = index.GetRole()
	newManifests = append(newManifests, &repository.Manifest{
		Signed: index,
	})

	// init snapshot
	snapshot := &repository.Snapshot{
		SignedBase: repository.SignedBase{
			Ty:          "snapshot",
			SpecVersion: "TODO",
			Expires:     currTime.UTC().Add(time.Hour * 24).Format(time.RFC3339), // 1d
			Version:     0,                                                       // not versioned
		},
	}
	snapshotManifest := &repository.Manifest{
		Signed: snapshot.SetVersions(newManifests),
	}
	root.Roles[snapshot.Filename()] = snapshot.GetRole()
	newManifests = append(newManifests, snapshotManifest)

	// init timestamp
	timestamp := &repository.Timestamp{
		SignedBase: repository.SignedBase{
			Ty:          "timestamp",
			SpecVersion: "TODO",
			Expires:     currTime.UTC().Add(time.Hour * 24).Format(time.RFC3339), // 1d
			Version:     uint(currTime.Unix()),
		},
	}
	timestamp, err := timestamp.SetSnapshot(snapshot)
	if err != nil {
		return err
	}
	timestampManifest := &repository.Manifest{
		Signed: timestamp,
	}
	root.Roles[timestamp.Filename()] = timestamp.GetRole()
	newManifests = append(newManifests, timestampManifest)

	// write to files
	for _, m := range newManifests {
		if err := repoManifests.SaveManifest(m); err != nil {
			return err
		}
	}
	return nil
}
