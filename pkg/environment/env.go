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
	"github.com/pingcap/tiup/pkg/repository/v0manifest"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/pingcap/tiup/pkg/verbose"
	"github.com/pingcap/tiup/pkg/version"
	"golang.org/x/mod/semver"
)

// EnvNameV0 is the name of the env var used to direct TiUp to use old manifests.
const EnvNameV0 = "TIUP_USE_V0"

// Name of components
const (
	TiUPName = "tiup"
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
	repo   *repository.Repository
	v1Repo *repository.V1Repository
}

// InitEnv creates a new Environment object configured using env vars and defaults. Uses the EnvNameV0 env var to
// determine whether to use v0 or v1 manifests.
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

	var repo *repository.Repository
	var v1repo *repository.V1Repository
	var err error

	if env := os.Getenv(EnvNameV0); env == "" || env == "disable" || env == "false" {
		var local v1manifest.LocalManifests
		local, err = v1manifest.NewManifests(profile)
		if err != nil {
			return nil, errors.Annotatef(err, "initial repository from mirror(%s) failed", mirrorAddr)
		}
		v1repo = repository.NewV1Repo(mirror, options, local)
	} else {
		repo, err = repository.NewRepository(mirror, options)
		if err != nil {
			return nil, errors.AddStack(err)
		}
	}

	verbose.Log("Initialize repository finished in %s", time.Since(initRepo))

	return &Environment{profile, repo, v1repo}, nil
}

// NewV0 creates a new Environment with the provided data. Note that environments created with this function do not
// support v1 repositories.
func NewV0(profile *localdata.Profile, repo *repository.Repository) *Environment {
	return &Environment{profile, repo, nil}
}

// Repository returns the initialized repository
func (env *Environment) Repository() *repository.Repository {
	return env.repo
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
	if env.v1Repo != nil {
		return nil
	}

	return env.repo.Close()
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
	if env.v1Repo != nil {
		var v1specs []repository.ComponentSpec
		for _, spec := range specs {
			component, v := ParseCompVersion(spec)
			if component == TiUPName {
				continue
			}
			v1specs = append(v1specs, repository.ComponentSpec{ID: component, Version: v.String(), Force: force, Nightly: nightly})
		}
		return env.v1Repo.UpdateComponents(v1specs)
	}

	manifest, err := env.latestManifest()
	if err != nil {
		return err
	}
	for _, spec := range specs {
		component, v := ParseCompVersion(spec)
		if component == TiUPName {
			continue
		}
		if nightly {
			v = version.NightlyVersion
		}
		if !manifest.HasComponent(component) {
			compInfo, found := manifest.FindComponent(component)
			if !found {
				return fmt.Errorf("component `%s` not found", component)
			}

			if !compInfo.IsSupport(env.PlatformString()) {
				return fmt.Errorf("component `%s` does not support `%s`", component, env.PlatformString())
			}
		}

		err := env.downloadComponent(component, v, v.IsNightly() || force)
		if err != nil {
			return err
		}
	}
	return nil
}

// PlatformString returns a string identifying the current system.
func (env *Environment) PlatformString() string {
	if env.v1Repo != nil {
		return env.v1Repo.PlatformString()
	}

	return repository.PlatformString(env.repo.GOOS, env.repo.GOARCH)
}

// SelfUpdate updates TiUp.
func (env *Environment) SelfUpdate() error {
	if env.v1Repo != nil {
		if err := env.v1Repo.DownloadTiup(env.LocalPath("bin")); err != nil {
			return err
		}

		// Cover the root.json from tiup.bar.gz
		return localdata.InitProfile().ResetMirror(Mirror(), "")
	}

	return env.repo.DownloadTiup(env.LocalPath("bin"))
}

func (env *Environment) downloadComponentv1(component string, version v0manifest.Version, overwrite bool) error {
	spec := repository.ComponentSpec{
		ID:      component,
		Version: string(version),
		Force:   overwrite,
	}

	return env.v1Repo.UpdateComponents([]repository.ComponentSpec{spec})
}

// downloadComponent downloads the specific version of a component from repository
func (env *Environment) downloadComponent(component string, version v0manifest.Version, overwrite bool) error {
	if env.v1Repo != nil {
		return env.downloadComponentv1(component, version, overwrite)
	}

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
		found, err := env.profile.VersionIsInstalled(component, version.String())
		if err != nil {
			return err
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
func (env *Environment) SelectInstalledVersion(component string, version v0manifest.Version) (v0manifest.Version, error) {
	return env.profile.SelectInstalledVersion(component, version)
}

// DownloadComponentIfMissing downloads the specific version of a component if it is missing
func (env *Environment) DownloadComponentIfMissing(component string, version v0manifest.Version) (v0manifest.Version, error) {
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
		version = v0manifest.Version(versions[len(versions)-1])
	}

	needDownload := version.IsEmpty()
	if !version.IsEmpty() {
		installed := false
		for _, v := range versions {
			if v0manifest.Version(v) == version {
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

// latestManifest returns the latest v0 component manifest and refresh the local cache
func (env *Environment) latestManifest() (*v0manifest.ComponentManifest, error) {
	manifest, err := env.repo.Manifest()
	if err != nil {
		return nil, err
	}
	if err := env.profile.SaveManifest(manifest); err != nil {
		return nil, err
	}
	return manifest, err
}

// GetComponentInstalledVersion return the installed version of component.
func (env *Environment) GetComponentInstalledVersion(component string, version v0manifest.Version) (v0manifest.Version, error) {
	return env.profile.GetComponentInstalledVersion(component, version)
}

// BinaryPath return the installed binary path.
func (env *Environment) BinaryPath(component string, version v0manifest.Version) (string, error) {
	if env.v1Repo != nil {
		installPath, err := env.profile.ComponentInstalledPath(component, version)
		if err != nil {
			return "", err
		}
		return env.v1Repo.BinaryPath(installPath, component, string(version))
	}

	return env.profile.BinaryPathV0(component, version)
}

// ParseCompVersion parses component part from <component>[:version] specification
func ParseCompVersion(spec string) (string, v0manifest.Version) {
	if strings.Contains(spec, ":") {
		parts := strings.SplitN(spec, ":", 2)
		return parts[0], v0manifest.Version(parts[1])
	}
	return spec, ""
}

// IsSupportedComponent return true if support if platform support the component.
func (env *Environment) IsSupportedComponent(component string) bool {
	if env.v1Repo != nil {
		// TODO
		return true
	}

	// check local manifest
	manifest := env.Profile().Manifest()
	if manifest != nil && manifest.HasComponent(component) {
		return true
	}

	manifest, err := env.Repository().Manifest()
	if err != nil {
		fmt.Println("Fetch latest manifest error:", err)
		return false
	}
	if err := env.Profile().SaveManifest(manifest); err != nil {
		fmt.Println("Save latest manifest error:", err)
	}
	comp, found := manifest.FindComponent(component)
	if !found {
		return false
	}
	return comp.IsSupport(env.PlatformString())
}
