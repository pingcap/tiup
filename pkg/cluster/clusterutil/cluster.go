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

package clusterutil

import (
	"os"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/repository"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/pingcap/tiup/pkg/utils"
)

// Repository exports interface to tiup-cluster
type Repository interface {
	DownloadComponent(comp, version, target string) error
	VerifyComponent(comp, version, target string) error
	ComponentBinEntry(comp, version string) (string, error)
}

type repositoryT struct {
	repo repository.Repository
}

// NewRepository returns repository
func NewRepository(os, arch string) (Repository, error) {
	repo := environment.GlobalEnv().V1Repository().WithOptions(repository.Options{
		GOOS:              os,
		GOARCH:            arch,
		DisableDecompress: true,
	})
	return &repositoryT{repo}, nil
}

func (r *repositoryT) DownloadComponent(comp, version, target string) error {
	versionItem, err := r.repo.ComponentVersion(comp, version, false)
	if err != nil {
		return err
	}

	return r.repo.DownloadComponent(versionItem, target)
}

func (r *repositoryT) VerifyComponent(comp, version, target string) error {
	versionItem, err := r.repo.ComponentVersion(comp, version, true)
	if err != nil {
		return err
	}

	file, err := os.Open(target)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := utils.CheckSHA256(file, versionItem.Hashes[v1manifest.SHA256]); err != nil {
		return errors.Errorf("validation failed for %s: %s", target, err)
	}
	return nil
}

func (r *repositoryT) ComponentBinEntry(comp, version string) (string, error) {
	versionItem, err := r.repo.ComponentVersion(comp, version, true)
	if err != nil {
		return "", err
	}

	return versionItem.Entry, nil
}
