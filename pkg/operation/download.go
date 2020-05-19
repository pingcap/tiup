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
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pingcap-incubator/tiup-cluster/pkg/meta"
	tiupmeta "github.com/pingcap-incubator/tiup/pkg/meta"
	"github.com/pingcap-incubator/tiup/pkg/repository"
	"github.com/pingcap-incubator/tiup/pkg/utils"
	"github.com/pingcap/errors"
)

// Download the specific version of a component from
// the repository, there is nothing to do if the specified version exists.
func Download(component, nodeOS, arch string, version repository.Version) error {
	if component == "" {
		return errors.New("component name not specified")
	}
	if version.IsEmpty() {
		return errors.Errorf("version not specified for component '%s'", component)
	}

	resName := fmt.Sprintf("%s-%s", component, version)
	fileName := fmt.Sprintf("%s-%s-%s.tar.gz", resName, nodeOS, arch)
	sha1File := fmt.Sprintf("%s-%s-%s.sha1", resName, nodeOS, arch)
	srcPath := meta.ProfilePath(meta.TiOpsPackageCacheDir, fileName)

	if err := os.MkdirAll(meta.ProfilePath(meta.TiOpsPackageCacheDir), 0755); err != nil {
		return err
	}

	// Download from repository if not exists
	if version.IsNightly() || utils.IsNotExist(srcPath) {
		options := repository.MirrorOptions{
			Progress: repository.DisableProgress{},
		}
		mirror := repository.NewMirror(tiupmeta.Mirror(), options)
		if err := mirror.Open(); err != nil {
			return errors.Trace(err)
		}
		defer mirror.Close()

		repo, err := repository.NewRepository(mirror, repository.Options{
			GOOS:              nodeOS,
			GOARCH:            arch,
			DisableDecompress: true,
		})
		if err != nil {
			return err
		}

		// validate component and platform info
		manifest, err := repo.Manifest()
		if err != nil {
			return err
		}
		compInfo, found := manifest.FindComponent(component)
		if !found {
			return errors.Errorf("component '%s' not supported", component)
		}
		if !compInfo.IsSupport(nodeOS, arch) {
			return errors.Errorf("component '%s' does not support platform %s/%s", component, nodeOS, arch)
		}

		versions, err := repo.ComponentVersions(component)
		if err != nil {
			return err
		}
		if !version.IsNightly() && !versions.ContainsVersion(version) {
			return errors.Errorf("component '%s' does not contain version '%s'", component, version)
		}

		// The /tmp maybe has different device from
		tmpDir := meta.ProfilePath(meta.TiOpsPackageCacheDir, "tmp")
		if err := os.MkdirAll(tmpDir, 0755); err != nil {
			return errors.Trace(err)
		}

		err = repo.Mirror().Download(fileName, tmpDir)
		if err != nil {
			return errors.AddStack(err)
		}

		err = repo.Mirror().Download(sha1File, tmpDir)
		if err != nil {
			return errors.AddStack(err)
		}

		tarPath := filepath.Join(tmpDir, fileName)
		shaPath := filepath.Join(tmpDir, sha1File)
		sha, err := ioutil.ReadFile(shaPath)
		if err != nil {
			return errors.Trace(err)
		}

		file, err := os.Open(tarPath)
		if err != nil {
			return errors.Trace(err)
		}

		err = utils.CheckSHA(file, string(sha))
		_ = file.Close()

		if err != nil {
			_ = os.Remove(tarPath)
			_ = os.Remove(shaPath)
			return err
		}

		if err = os.Rename(tarPath, srcPath); err != nil {
			return errors.Trace(err)
		}

		if err = os.Rename(shaPath, meta.ProfilePath(meta.TiOpsPackageCacheDir, sha1File)); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}
