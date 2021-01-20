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

package environment

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/repository"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	pkgver "github.com/pingcap/tiup/pkg/repository/version"
	"github.com/pingcap/tiup/pkg/verbose"
	"golang.org/x/mod/semver"
)

// Name of components
const (
	tiupName = "tiup"
)

// Mirror return mirror of tiup.
// If it's not defined, it will use "https://tiup-mirrors.pingcap.com/".
func Mirror() string {
	profile := localdata.InitProfile()
	cfg := profile.Config

	reset := func(m string) {
		os.Setenv(repository.EnvMirrors, m)
		if err := profile.ResetMirror(m, ""); err != nil {
			fmt.Printf("WARNING: reset mirror failed, %s\n", err.Error())
		}
	}

	m := os.Getenv(repository.EnvMirrors)
	if m != "" {
		if cfg.Mirror != m {
			fmt.Printf(`WARNING: both mirror config(%s)
and TIUP_MIRRORS(%s) have been set.
Setting mirror to TIUP_MIRRORS(%s)
`, cfg.Mirror, m, m)
			reset(m)
		}
		return m
	} else if cfg.Mirror != "" {
		os.Setenv(repository.EnvMirrors, cfg.Mirror)
		return cfg.Mirror
	}

	return repository.DefaultMirror
}

// Environment is the user's fundamental configuration including local and remote parts.
type Environment struct {
	// profile represents the TiUP local profile
	profile *localdata.Profile
	// repo represents the components repository of TiUP, it can be a
	// local file system or a HTTP URL
	v1Repo *repository.V1Repository
}

// InitEnv creates a new Environment object configured using env vars and defaults.
func InitEnv(options repository.Options) (*Environment, error) {
	if env := GlobalEnv(); env != nil {
		return env, nil
	}

	initRepo := time.Now()
	profile := localdata.InitProfile()

	// Initialize the repository
	// Replace the mirror if some sub-commands use different mirror address
	mirrorAddr := Mirror()
	mirror := repository.NewMirror(mirrorAddr, repository.MirrorOptions{})
	if err := mirror.Open(); err != nil {
		return nil, err
	}

	var v1repo *repository.V1Repository
	var err error

	var local v1manifest.LocalManifests
	local, err = v1manifest.NewManifests(profile)
	if err != nil {
		return nil, errors.Annotatef(err, "initial repository from mirror(%s) failed", mirrorAddr)
	}
	v1repo = repository.NewV1Repo(mirror, options, local)

	verbose.Log("Initialize repository finished in %s", time.Since(initRepo))

	return &Environment{profile, v1repo}, nil
}

// V1Repository returns the initialized v1 repository
func (env *Environment) V1Repository() *repository.V1Repository {
	return env.v1Repo
}

// Profile returns the profile of local data
func (env *Environment) Profile() *localdata.Profile {
	return env.profile
}

// Close release resource of env.
func (env *Environment) Close() error {
	// no need for v1manifest
	return nil
}

// SetProfile exports for test
func (env *Environment) SetProfile(p *localdata.Profile) {
	env.profile = p
}

// LocalPath returns the local path absolute path
func (env *Environment) LocalPath(path ...string) string {
	return env.profile.Path(filepath.Join(path...))
}

// UpdateComponents updates or installs all components described by specs.
func (env *Environment) UpdateComponents(specs []string, nightly, force bool) error {
	var v1specs []repository.ComponentSpec
	for _, spec := range specs {
		component, v := ParseCompVersion(spec)
		if component == tiupName {
			continue
		}
		v1specs = append(v1specs, repository.ComponentSpec{ID: component, Version: v.String(), Force: force, Nightly: nightly})
	}
	return env.v1Repo.UpdateComponents(v1specs)
}

// PlatformString returns a string identifying the current system.
func (env *Environment) PlatformString() string {
	return env.v1Repo.PlatformString()
}

// SelfUpdate updates TiUP.
func (env *Environment) SelfUpdate() error {
	if err := env.v1Repo.DownloadTiUP(env.LocalPath("bin")); err != nil {
		return err
	}

	// Cover the root.json from tiup.bar.gz
	return localdata.InitProfile().ResetMirror(Mirror(), "")
}

func (env *Environment) downloadComponentv1(component string, version pkgver.Version, overwrite bool) error {
	spec := repository.ComponentSpec{
		ID:      component,
		Version: string(version),
		Force:   overwrite,
	}

	return env.v1Repo.UpdateComponents([]repository.ComponentSpec{spec})
}

// downloadComponent downloads the specific version of a component from repository
func (env *Environment) downloadComponent(component string, version pkgver.Version, overwrite bool) error {
	return env.downloadComponentv1(component, version, overwrite)
}

// SelectInstalledVersion selects the installed versions and the latest release version
// will be chosen if there is an empty version
func (env *Environment) SelectInstalledVersion(component string, version pkgver.Version) (pkgver.Version, error) {
	return env.profile.SelectInstalledVersion(component, version)
}

// DownloadComponentIfMissing downloads the specific version of a component if it is missing
func (env *Environment) DownloadComponentIfMissing(component string, version pkgver.Version) (pkgver.Version, error) {
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
		version = pkgver.Version(versions[len(versions)-1])
	}

	needDownload := version.IsEmpty()
	if !version.IsEmpty() {
		installed := false
		for _, v := range versions {
			if pkgver.Version(v) == version {
				installed = true
				break
			}
		}
		needDownload = !installed
	}

	if needDownload {
		fmt.Printf("The component `%s` is not installed; downloading from repository.\n", component)
		err := env.downloadComponent(component, version, false)
		if err != nil {
			return "", err
		}
	}

	if version.IsEmpty() {
		return env.SelectInstalledVersion(component, version)
	}

	return version, nil
}

// GetComponentInstalledVersion return the installed version of component.
func (env *Environment) GetComponentInstalledVersion(component string, version pkgver.Version) (pkgver.Version, error) {
	return env.profile.GetComponentInstalledVersion(component, version)
}

// BinaryPath return the installed binary path.
func (env *Environment) BinaryPath(component string, version pkgver.Version) (string, error) {
	installPath, err := env.profile.ComponentInstalledPath(component, version)
	if err != nil {
		return "", err
	}
	return env.v1Repo.BinaryPath(installPath, component, string(version))
}

// ParseCompVersion parses component part from <component>[:version] specification
func ParseCompVersion(spec string) (string, pkgver.Version) {
	if strings.Contains(spec, ":") {
		parts := strings.SplitN(spec, ":", 2)
		return parts[0], pkgver.Version(parts[1])
	}
	return spec, ""
}
