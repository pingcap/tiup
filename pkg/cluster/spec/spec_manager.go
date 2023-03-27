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
	"errors"
	"os"
	"path/filepath"

	"github.com/joomcode/errorx"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/tui"
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
	// ErrSaveScaleOutFileFailed is ErrSaveMetaFailed
	ErrSaveScaleOutFileFailed = errNS.NewType("save_scale-out_lock_failed")
)

const (
	// metaFileName is the file name of the meta file.
	metaFileName = "meta.yaml"
	// PatchDirName is the directory to store patch file eg. {PatchDirName}/tidb-hotfix.tar.gz
	PatchDirName = "patch"
	// BackupDirName is the directory to save backup files.
	BackupDirName = "backup"
	// ScaleOutLockName scale_out snapshot file, like file lock
	ScaleOutLockName = ".scale-out.yaml"
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
		// keep the same behavior with legacy version of TiUP, we could change
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

	if err := utils.MkdirAll(backupDir, 0755); err != nil {
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

	err = utils.SaveFileWithBackup(metaFile, data, backupDir)
	if err != nil {
		return wrapError(err)
	}

	return nil
}

// Metadata tries to read the metadata of a cluster from file
func (s *SpecManager) Metadata(clusterName string, meta any) error {
	fname := s.Path(clusterName, metaFileName)

	yamlFile, err := os.ReadFile(fname)
	if err != nil {
		return perrs.AddStack(err)
	}

	err = yaml.Unmarshal(yamlFile, meta)
	if err != nil {
		return perrs.AddStack(err)
	}

	return nil
}

// Exist check if the cluster exist by checking the meta file.
func (s *SpecManager) Exist(clusterName string) (exist bool, err error) {
	fname := s.Path(clusterName, metaFileName)

	_, err = os.Stat(fname)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, perrs.AddStack(err)
	}

	return true, nil
}

// Remove remove the data with specified cluster name.
func (s *SpecManager) Remove(clusterName string) error {
	return os.RemoveAll(s.Path(clusterName))
}

// List return the cluster names.
func (s *SpecManager) List() (clusterNames []string, err error) {
	fileInfos, err := os.ReadDir(s.base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, perrs.AddStack(err)
	}

	for _, info := range fileInfos {
		if utils.IsNotExist(s.Path(info.Name(), metaFileName)) {
			continue
		}
		clusterNames = append(clusterNames, info.Name())
	}

	return
}

// GetAllClusters get a metadata list of all clusters deployed by current user
func (s *SpecManager) GetAllClusters() (map[string]Metadata, error) {
	clusters := make(map[string]Metadata)
	names, err := s.List()
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		metadata := s.NewMetadata()
		err = s.Metadata(name, metadata)
		// clusters with topology validation errors should also be listed
		if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) &&
			!errors.Is(perrs.Cause(err), ErrNoTiSparkMaster) {
			return nil, perrs.Trace(err)
		}
		clusters[name] = metadata
	}
	return clusters, nil
}

// ensureDir ensures that the cluster directory exists.
func (s *SpecManager) ensureDir(clusterName string) error {
	if err := utils.MkdirAll(s.Path(clusterName), 0755); err != nil {
		return ErrCreateDirFailed.
			Wrap(err, "Failed to create cluster metadata directory '%s'", s.Path(clusterName)).
			WithProperty(tui.SuggestionFromString("Please check file system permissions and try again."))
	}
	return nil
}

// ScaleOutLock tries to read the ScaleOutLock of a cluster from file
func (s *SpecManager) ScaleOutLock(clusterName string) (Topology, error) {
	if locked, err := s.IsScaleOutLocked(clusterName); !locked {
		return nil, ErrSaveScaleOutFileFailed.Wrap(err, "Scale-out file lock does not exist").
			WithProperty(tui.SuggestionFromString("Please make sure to run tiup-cluster scale-out --stage1 and try again."))
	}

	fname := s.Path(clusterName, ScaleOutLockName)

	// UnMarshal file lock
	topo := &Specification{}
	err := ParseTopologyYaml(fname, topo)
	if err != nil {
		return nil, err
	}
	return topo, nil
}

// ScaleOutLockedErr: Determine whether there is a lock, and report an error if it exists
func (s *SpecManager) ScaleOutLockedErr(clusterName string) error {
	if locked, err := s.IsScaleOutLocked(clusterName); locked {
		return errNS.NewType("scale-out lock").Wrap(err, "Scale-out file lock already exists").
			WithProperty(tui.SuggestionFromString("Please run 'tiup-cluster scale-out --stage2' to continue."))
	}
	return nil
}

// IsScaleOutLocked:  judge the cluster scale-out file lock status
func (s *SpecManager) IsScaleOutLocked(clusterName string) (locked bool, err error) {
	fname := s.Path(clusterName, ScaleOutLockName)

	_, err = os.Stat(fname)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, perrs.AddStack(err)
	}

	return true, nil
}

// NewScaleOutLock save the meta with specified cluster name.
func (s *SpecManager) NewScaleOutLock(clusterName string, topo Topology) error {
	wrapError := func(err error) *errorx.Error {
		return ErrSaveScaleOutFileFailed.Wrap(err, "Failed to create scale-out file lock")
	}

	if locked, err := s.IsScaleOutLocked(clusterName); locked {
		return wrapError(err).
			WithProperty(tui.SuggestionFromString("The scale out file lock already exists, please run tiup-cluster scale-out --stage2 to continue."))
	}

	lockFile := s.Path(clusterName, ScaleOutLockName)

	if err := s.ensureDir(clusterName); err != nil {
		return wrapError(err)
	}

	data, err := yaml.Marshal(topo)
	if err != nil {
		return wrapError(err)
	}

	err = utils.WriteFile(lockFile, data, 0644)
	if err != nil {
		return wrapError(err)
	}

	return nil
}

// ReleaseScaleOutLock remove the scale-out file lock with specified cluster
func (s *SpecManager) ReleaseScaleOutLock(clusterName string) error {
	return os.Remove(s.Path(clusterName, ScaleOutLockName))
}
