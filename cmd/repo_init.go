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
			if len(args) == 0 {
				if repoPath, err = os.Getwd(); err != nil {
					return err
				}
			} else {
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

	return cmd
}

func initRepo(path string) error {
	currTime := time.Now()
	repoManifests := repository.NewManifests(path)
	// TODO: set key store

	// initial manifests
	newManifests := make([]*repository.Manifest, 0)

	// init the root manifest
	newManifests = append(newManifests, &repository.Manifest{
		Signed: &repository.Root{
			SignedBase: repository.SignedBase{
				Ty:          "root",
				SpecVersion: "TODO",
				Expires:     currTime.UTC().Add(time.Hour * 24 * 365).Format(time.RFC3339), // 1y
				Version:     1,                                                             // initial repo starts with version 1
			},
		},
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
	newManifests = append(newManifests, &repository.Manifest{
		Signed: snapshot.SetVersions(newManifests),
	})

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
	newManifests = append(newManifests, &repository.Manifest{
		Signed: timestamp,
	})

	// write to files
	for _, m := range newManifests {
		if err := repoManifests.SaveManifest(m); err != nil {
			return err
		}
	}
	return nil
}
