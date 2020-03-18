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
	"os/user"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pingcap-incubator/tiup/pkg/localdata"
	"github.com/pingcap-incubator/tiup/pkg/repository"
	"github.com/pingcap/errors"
	"golang.org/x/mod/semver"
)

const defaultMirror = "https://tiup-mirrors.pingcap.com/"

var (
	// profile represents the TiUP local profile
	profile *localdata.Profile
	// repo represents the components repository of TiUP, it can be a
	// local file system or a HTTP URL
	repo *repository.Repository
)

func init() {
	// Initialize the global profile
	var profileDir string
	switch {
	case os.Getenv(localdata.EnvNameHome) != "":
		profileDir = os.Getenv(localdata.EnvNameHome)
	case localdata.DefaultTiupHome != "":
		profileDir = localdata.DefaultTiupHome
	default:
		u, err := user.Current()
		if err != nil {
			panic("cannot get current user information: " + err.Error())
		}
		profileDir = filepath.Join(u.HomeDir, localdata.ProfileDirName)
	}
	profile = localdata.NewProfile(profileDir)
}

func mirror() string {
	if m := os.Getenv("TIUP_MIRRORS"); m != "" {
		return m
	}
	return defaultMirror
}

// InitRepository initialize the repository
func InitRepository(options repository.Options) error {
	// Initialize the repository
	// Replace the mirror if some sub-commands use different mirror address
	mirror := repository.NewMirror(mirror())
	if err := mirror.Open(); err != nil {
		return err
	}
	repo = repository.NewRepository(mirror, options)
	return nil
}

// Repository returns the initialized repository
func Repository() *repository.Repository {
	return repo
}

// SetRepository exports for test
func SetRepository(r *repository.Repository) {
	repo = r
}

// Profile returns the profile of local data
func Profile() *localdata.Profile {
	return profile
}

// SetProfile exports for test
func SetProfile(p *localdata.Profile) {
	profile = p
}

// LocalPath returns the local path absolute path
func LocalPath(path ...string) string {
	return profile.Path(filepath.Join(path...))
}

// LocalRoot returns the root path of profile
func LocalRoot() string {
	return profile.Root()
}

// ParseCompVersion parses component part from <component>[:version] specification
func ParseCompVersion(spec string) (string, repository.Version) {
	if strings.Contains(spec, ":") {
		parts := strings.SplitN(spec, ":", 2)
		return parts[0], repository.Version(parts[1])
	}
	return spec, ""
}

// BinaryPath returns the binary path of component specific version
func BinaryPath(component string, version repository.Version) (string, error) {
	manifest := profile.Versions(component)
	if manifest == nil {
		return "", errors.Errorf("component `%s` doesn't install", component)
	}
	var entry string
	if version.IsNightly() && manifest.Nightly != nil {
		entry = manifest.Nightly.Entry
	} else {
		for _, v := range manifest.Versions {
			if v.Version == version {
				entry = v.Entry
			}
		}
	}
	if entry == "" {
		return "", errors.Errorf("cannot found entry for %s:%s", component, version)
	}
	installPath, err := ComponentInstalledDir(component, version)
	if err != nil {
		return "", err
	}
	return filepath.Join(installPath, entry), nil
}

// ComponentInstalledDir returns the path where the component installed
func ComponentInstalledDir(component string, version repository.Version) (string, error) {
	versions, err := profile.InstalledVersions(component)
	if err != nil {
		return "", err
	}

	// Use the latest version if user doesn't specify a specific version
	// report an error if the specific component doesn't be installed

	// Check whether the specific version exist in local
	if version.IsEmpty() && len(versions) > 0 {
		sort.Slice(versions, func(i, j int) bool {
			return semver.Compare(versions[i], versions[j]) < 0
		})
		version = repository.Version(versions[len(versions)-1])
	} else if version.IsEmpty() {
		return "", fmt.Errorf("component not installed, please try `tiup install %s` to install it", component)
	}
	return filepath.Join(LocalPath(localdata.ComponentParentDir), component, version.String()), nil
}

// DownloadComponent downloads the specific version of a component from repository
func DownloadComponent(component string, version repository.Version, overwrite bool) error {
	versions, err := repo.ComponentVersions(component)
	if err != nil {
		return err
	}
	err = profile.SaveVersions(component, versions)
	if err != nil {
		return err
	}
	if version.IsNightly() && versions.Nightly == nil {
		fmt.Printf("The component `%s` has not nightly version, skiped\n", component)
		return nil
	}
	if version.IsEmpty() {
		version = versions.LatestVersion()
	}
	if !overwrite {
		// Ignore if installed
		installed, err := profile.InstalledVersions(component)
		if err != nil {
			return err
		}
		found := false
		for _, v := range installed {
			if repository.Version(v) == version {
				found = true
				break
			}
		}
		if found {
			fmt.Printf("The `%s:%s` has been installed\n", component, version)
			return nil
		}
	}

	return repo.DownloadComponent(LocalPath(localdata.ComponentParentDir), component, version)
}

// SelectInstalledVersion selects the installed versions and the latest release version
// will be chosen if there is an empty version
func SelectInstalledVersion(component string, version repository.Version) (repository.Version, error) {
	installed, err := profile.InstalledVersions(component)
	if err != nil {
		return "", err
	}

	errInstallFirst := fmt.Errorf("use `tiup install %[1]s` to install `%[1]s` first", component)
	if len(installed) < 1 {
		return "", errInstallFirst
	}

	if version.IsEmpty() {
		sort.Slice(installed, func(i, j int) bool {
			return semver.Compare(installed[i], installed[j]) < 0
		})
		version = repository.Version(installed[len(installed)-1])
	}
	found := false
	for _, v := range installed {
		if repository.Version(v) == version {
			found = true
			break
		}
	}
	if !found {
		return "", errInstallFirst
	}
	return version, nil
}

// DownloadComponentIfMissing downloads the specific version of a component if it is missing
func DownloadComponentIfMissing(component string, version repository.Version) (repository.Version, error) {
	versions, err := profile.InstalledVersions(component)
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
		version = repository.Version(versions[len(versions)-1])
	}

	needDownload := version.IsEmpty()
	if !version.IsEmpty() {
		installed := false
		for _, v := range versions {
			if repository.Version(v) == version {
				installed = true
				break
			}
		}
		needDownload = !installed
	}

	if needDownload {
		fmt.Printf("The component `%s` doesn't installed, download from repository\n", component)
		err := DownloadComponent(component, version, false)
		if err != nil {
			return "", err
		}
	}

	if version.IsEmpty() {
		return SelectInstalledVersion(component, version)
	}

	return version, nil
}

// LatestManifest returns the latest component manifest and refresh the local cache
func LatestManifest() (*repository.ComponentManifest, error) {
	manifest, err := repo.Manifest()
	if err != nil {
		return nil, err
	}
	if err := profile.SaveManifest(manifest); err != nil {
		return nil, err
	}
	return manifest, err
}
