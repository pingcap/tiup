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

package repository

import (
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/pingcap/tiup/pkg/utils"
)

// Repository represents a local components repository that mirrored the remote Repository(either filesystem or HTTP server).
type Repository interface {
	UpdateComponents(specs []ComponentSpec) error
	ResolveComponentVersion(id, constraint string) (utils.Version, error)
	ComponentInstalled(component, version string) (bool, error)
	BinaryPath(installPath string, componentID string, ver string) (string, error)
	DownloadTiUP(targetDir string) error
	UpdateComponentManifests() error
	LatestStableVersion(id string, withYanked bool) (utils.Version, *v1manifest.VersionItem, error)
	LocalLoadManifest(index *v1manifest.Index) (*v1manifest.Manifest, bool, error)
	LocalLoadComponentManifest(component *v1manifest.ComponentItem, filename string) (*v1manifest.Component, error)
	LocalComponentManifest(id string, withYanked bool) (com *v1manifest.Component, err error)
	GetComponentManifest(id string, withYanked bool) (com *v1manifest.Component, err error)
	Mirror() Mirror
	FetchIndexManifest() (index *v1manifest.Index, err error)
	FetchRootManifest() (root *v1manifest.Root, err error)
	PurgeTimestamp()
	LatestNightlyVersion(id string) (utils.Version, *v1manifest.VersionItem, error)
	WithOptions(opts Options) Repository
}

// Options represents options for a repository
type Options struct {
	GOOS              string
	GOARCH            string
	DisableDecompress bool
}
