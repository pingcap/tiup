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
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pingcap-incubator/tiup/pkg/localdata"
	"github.com/pingcap-incubator/tiup/pkg/repository"
	"github.com/pingcap-incubator/tiup/pkg/repository/manifest"
	"golang.org/x/mod/semver"
)

// Mirror return mirror of tiup.
// If it's not defined, it will use "https://tiup-mirrors.pingcap.com/".
func Mirror() string {
	if m := os.Getenv(repository.EnvMirrors); m != "" {
		return m
	}
	return repository.DefaultMirror
}

// Environment is the user's fundamental configuration including local and remote parts.
type Environment struct {
	// profile represents the TiUP local profile
	profile *localdata.Profile
	// repo represents the components repository of TiUP, it can be a
	// local file system or a HTTP URL
	repo *repository.Repository
}

// InitEnv creates a new Environment object configured using env vars and defaults.
func InitEnv(options repository.Options) (*Environment, error) {
	profile := localdata.InitProfile()

	// Initialize the repository
	// Replace the mirror if some sub-commands use different mirror address
	mirror := repository.NewMirror(Mirror(), repository.MirrorOptions{})
	repo, err := repository.NewRepository(mirror, options)
	return &Environment{profile, repo}, err
}

// New creates a new Environment with the provided data.
func New(profile *localdata.Profile, repo *repository.Repository) *Environment {
	return &Environment{profile, repo}
}

// Repository returns the initialized repository
func (env *Environment) Repository() *repository.Repository {
	return env.repo
}

// SetRepository exports for test
func (env *Environment) SetRepository(r *repository.Repository) {
	env.repo = r
}

// Profile returns the profile of local data
func (env *Environment) Profile() *localdata.Profile {
	return env.profile
}

// SetProfile exports for test
func (env *Environment) SetProfile(p *localdata.Profile) {
	env.profile = p
}

// LocalPath returns the local path absolute path
func (env *Environment) LocalPath(path ...string) string {
	return env.profile.Path(filepath.Join(path...))
}

// DownloadComponent downloads the specific version of a component from repository
func (env *Environment) DownloadComponent(component string, version manifest.Version, overwrite bool) error {
	versions, err := env.repo.ComponentVersions(component)
	if err != nil {
		return err
	}
	err = env.profile.SaveVersions(component, versions)
	if err != nil {
		return err
	}
	if version.IsNightly() && versions.Nightly == nil {
		fmt.Printf("The component `%s` does not have a nightly version; skipped.\n", component)
		return nil
	}
	if version.IsEmpty() {
		version = versions.LatestVersion()
	}
	if !overwrite {
		// Ignore if installed
		installed, err := env.profile.InstalledVersions(component)
		if err != nil {
			return err
		}
		found := false
		for _, v := range installed {
			if manifest.Version(v) == version {
				found = true
				break
			}
		}
		if found {
			fmt.Printf("The component `%s:%s` has been installed.\n", component, version)
			return nil
		}
	}

	return env.repo.DownloadComponent(env.LocalPath(localdata.ComponentParentDir), component, version)
}

// SelectInstalledVersion selects the installed versions and the latest release version
// will be chosen if there is an empty version
func (env *Environment) SelectInstalledVersion(component string, version manifest.Version) (manifest.Version, error) {
	return env.profile.SelectInstalledVersion(component, version)
}

// DownloadComponentIfMissing downloads the specific version of a component if it is missing
func (env *Environment) DownloadComponentIfMissing(component string, version manifest.Version) (manifest.Version, error) {
	versions, err := env.profile.InstalledVersions(component)
	if err != nil {
		return "", err
	}

	// Use the latest version if user doesn't specify a specific version and
	// download the latest version if the specific component doesn't be installed

	// Check whether the specific version exist in local
	if version.IsEmpty() && len(versions) > 0 {
		sort.Slice(versions, func(i, j int) bool {
			return semver.Compare(versions[i], versions[j]) < 0
		})
		version = manifest.Version(versions[len(versions)-1])
	}

	needDownload := version.IsEmpty()
	if !version.IsEmpty() {
		installed := false
		for _, v := range versions {
			if manifest.Version(v) == version {
				installed = true
				break
			}
		}
		needDownload = !installed
	}

	if needDownload {
		fmt.Printf("The component `%s` is not installed; downloading from repository.\n", component)
		err := env.DownloadComponent(component, version, false)
		if err != nil {
			return "", err
		}
	}

	if version.IsEmpty() {
		return env.SelectInstalledVersion(component, version)
	}

	return version, nil
}

// LatestManifest returns the latest component manifest and refresh the local cache
func (env *Environment) LatestManifest() (*manifest.ComponentManifest, error) {
	manifest, err := env.repo.Manifest()
	if err != nil {
		return nil, err
	}
	if err := env.profile.SaveManifest(manifest); err != nil {
		return nil, err
	}
	return manifest, err
}

// ParseCompVersion parses component part from <component>[:version] specification
func ParseCompVersion(spec string) (string, manifest.Version) {
	if strings.Contains(spec, ":") {
		parts := strings.SplitN(spec, ":", 2)
		return parts[0], manifest.Version(parts[1])
	}
	return spec, ""
}
