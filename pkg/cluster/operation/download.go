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

package operator

import (
	"fmt"
	"os"

	"github.com/pingcap/tiup/pkg/cluster/clusterutil"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/utils"
	tiupver "github.com/pingcap/tiup/pkg/version"
)

// Download the specific version of a component from
// the repository, there is nothing to do if the specified version exists.
func Download(component, nodeOS, arch string, version string) error {
	if component == "" {
		return errors.New("component name not specified")
	}
	if version == "" {
		return errors.Errorf("version not specified for component '%s'", component)
	}

	resName := fmt.Sprintf("%s-%s", component, version)
	fileName := fmt.Sprintf("%s-%s-%s.tar.gz", resName, nodeOS, arch)
	srcPath := spec.ProfilePath(spec.TiOpsPackageCacheDir, fileName)

	if err := os.MkdirAll(spec.ProfilePath(spec.TiOpsPackageCacheDir), 0755); err != nil {
		return err
	}

	repo, err := clusterutil.NewRepository(nodeOS, arch)
	if err != nil {
		return err
	}

	if utils.IsExist(srcPath) {
		if err := repo.VerifyComponent(component, version, srcPath); err != nil {
			os.Remove(srcPath)
		}
	}

	// Download from repository if not exists
	if version == tiupver.NightlyVersion || utils.IsNotExist(srcPath) {
		if err := repo.DownloadComponent(component, version, srcPath); err != nil {
			return err
		}
	}
	return nil
}
