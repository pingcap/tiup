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

package spec

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/joomcode/errorx"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cliutil"
	"github.com/pingcap/tiup/pkg/file"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/pingcap/tiup/pkg/version"
	"gopkg.in/yaml.v2"
)

var (
	errNS = errorx.NewNamespace("spec")
	// ErrCreateDirFailed is ErrCreateDirFailed
	ErrCreateDirFailed = errNS.NewType("create_dir_failed")
	// ErrSaveMetaFailed is ErrSaveMetaFailed
	ErrSaveMetaFailed = errNS.NewType("save_meta_failed")
)

const (
	// metaFileName is the file name of the meta file.
	metaFileName = "meta.yaml"
	// PatchDirName is the directory to store patch file eg. {PatchDirName}/tidb-hotfix.tar.gz
	PatchDirName = "patch"
	// BackupDirName is the directory to save backup files.
	BackupDirName = "backup"
)

//revive:disable

// SpecManager control management of spec meta data.
type SpecManager struct {
	base    string
	newMeta func() Metadata
}

// NewSpec create a spec instance.
func NewSpec(base string, newMeta func() Metadata) *SpecManager {
	return &SpecManager{
		base:    base,
		newMeta: newMeta,
	}
}

// NewMetadata alloc a Metadata according the type.
func (s *SpecManager) NewMetadata() Metadata {
	return s.newMeta()
}

// Path returns the full path to a subpath (file or directory) of a
// cluster, it is a subdir in the profile dir of the user, with the cluster name
// as its name.
func (s *SpecManager) Path(cluster string, subpath ...string) string {
	if cluster == "" {
		// keep the same behavior with legacy version of TiOps, we could change
		// it in the future if needed.
		cluster = "default-cluster"
	}

	return filepath.Join(append([]string{
		s.base,
		cluster,
	}, subpath...)...)
}

// SaveMeta save the meta with specified cluster name.
func (s *SpecManager) SaveMeta(clusterName string, meta Metadata) error {
	wrapError := func(err error) *errorx.Error {
		return ErrSaveMetaFailed.Wrap(err, "Failed to save cluster metadata")
	}

	metaFile := s.Path(clusterName, metaFileName)
	backupDir := s.Path(clusterName, BackupDirName)

	if err := s.ensureDir(clusterName); err != nil {
		return wrapError(err)
	}

	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return wrapError(err)
	}

	data, err := yaml.Marshal(meta)
	if err != nil {
		return wrapError(err)
	}

	opsVer := meta.GetBaseMeta().OpsVer
	if opsVer != nil {
		*opsVer = version.NewTiUPVersion().String()
	}

	err = file.SaveFileWithBackup(metaFile, data, backupDir)
	if err != nil {
		return wrapError(err)
	}

	return nil
}

// Metadata tries to read the metadata of a cluster from file
func (s *SpecManager) Metadata(clusterName string, meta interface{}) error {
	fname := s.Path(clusterName, metaFileName)

	yamlFile, err := ioutil.ReadFile(fname)
	if err != nil {
		return errors.AddStack(err)
	}

	err = yaml.Unmarshal(yamlFile, meta)
	if err != nil {
		return errors.AddStack(err)
	}

	return nil
}

// Exist check if the cluster exist by checking the meta file.
func (s *SpecManager) Exist(name string) (exist bool, err error) {
	fname := s.Path(name, metaFileName)

	_, err = os.Stat(fname)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, errors.AddStack(err)
	}

	return true, nil
}

// Remove remove the data with specified cluster name.
func (s *SpecManager) Remove(name string) error {
	return os.RemoveAll(s.Path(name))
}

// List return the cluster names.
func (s *SpecManager) List() (names []string, err error) {
	fileInfos, err := ioutil.ReadDir(s.base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, errors.AddStack(err)
	}

	for _, info := range fileInfos {
		if utils.IsNotExist(s.Path(info.Name(), metaFileName)) {
			continue
		}
		names = append(names, info.Name())
	}

	return
}

// ensureDir ensures that the cluster directory exists.
func (s *SpecManager) ensureDir(clusterName string) error {
	if err := utils.CreateDir(s.Path(clusterName)); err != nil {
		return ErrCreateDirFailed.
			Wrap(err, "Failed to create cluster metadata directory '%s'", s.Path(clusterName)).
			WithProperty(cliutil.SuggestionFromString("Please check file system permissions and try again."))
	}
	return nil
}
