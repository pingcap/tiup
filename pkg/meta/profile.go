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

package meta

import (
	"os"
	"os/user"
	"path"

	"github.com/pingcap-incubator/tiops/pkg/utils"
	tiuplocaldata "github.com/pingcap-incubator/tiup/pkg/localdata"
	"github.com/pingcap/errors"
)

const (
	TiOpsPackageCacheDir = "packages"
)

var profileDir string

// getHomeDir get the home directory of current user (if they have one).
// The result path might be empty.
func getHomeDir() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", errors.Trace(err)
	}
	return u.HomeDir, nil
}

// Initialize initializes the global variables of meta package
func Initialize() error {
	tiupData := os.Getenv(tiuplocaldata.EnvNameComponentDataDir)
	if tiupData == "" {
		homeDir, err := getHomeDir()
		if err != nil {
			return errors.Trace(err)
		}
		profileDir = path.Join(homeDir, ".tiops")
	} else {
		profileDir = tiupData
	}

	// make sure the dir exist
	return utils.CreateDir(profileDir)
}

// ProfileDir returns the full profile directory path of TiOps. If the
// environment variable TIUP_COMPONENT_DATA_DIR is set, it is used as root of
// the profile directory, otherwise the `$HOME/.tiops` of current user is used.
// The directory will be created before return if it does not already exist.
func ProfileDir() string {
	return profileDir
}

func ProfilePath(subpath ...string) string {
	return path.Join(append([]string{profileDir}, subpath...)...)
}

// ClusterPath returns the full path to a subpath (file or directory) of a
// cluster, it is a subdir in the profile dir of the user, with the cluster name
// as its name.
// It is not garenteed the path already exist.
func ClusterPath(cluster string, subpath ...string) string {
	if cluster == "" {
		// keep the same behavior with legancy version of TiOps, we could change
		// it in the future if needed.
		cluster = "default-cluster"
	}

	return path.Join(append([]string{profileDir, cluster}, subpath...)...)
}
