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

package localdata

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/utils"
	"golang.org/x/mod/semver"
)

// Profile represents the `one tiup mirror` profile
type Profile struct {
	root   string
	name   string
	Config *TiUPConfig
}

// NewProfile returns a new profile instance
func NewProfile(root, name string, config *TiUPConfig) *Profile {
	return &Profile{root: root, name: name, Config: config}
}

// InitProfile creates a new profile using environment variables and defaults.
func InitProfile() *Profile {
	var profileDir string
	switch {
	case os.Getenv(EnvNameHome) != "":
		profileDir = os.Getenv(EnvNameHome)
	case DefaultTiUPHome != "":
		profileDir = DefaultTiUPHome
	default:
		u, err := user.Current()
		if err != nil {
			panic("cannot get current user information: " + err.Error())
		}
		profileDir = filepath.Join(u.HomeDir, ProfileDirName)
	}

	cfg, err := InitConfig(profileDir)
	if err != nil {
		panic("cannot read config: " + err.Error())
	}
	return NewProfile(profileDir, "../bin", cfg)
}

// Path returns a full path which is related to profile root directory
func (p *Profile) Path(relpath ...string) string {
	return filepath.Join(append([]string{p.root}, relpath...)...)
}

// Root returns the root path of the `tiup`
func (p *Profile) Root() string {
	return p.root
}

func (p *Profile) Name() string {
	return p.name
}

// GetComponentInstalledVersion return the installed version of component.
func (p *Profile) GetComponentInstalledVersion(component string, ver utils.Version) (utils.Version, error) {
	if !ver.IsEmpty() && ver.String() != utils.NightlyVersionAlias {
		return ver, nil
	}
	versions, err := p.InstalledVersions(component)
	if err != nil {
		return "", err
	}

	// Use the latest version if user doesn't specify a specific version
	// report an error if the specific component doesn't be installed

	// Check whether the specific version exist in local
	if len(versions) == 0 {
		return "", errors.Errorf("component not installed, please try `tiup install %s` to install it", component)
	}
	sort.Slice(versions, func(i, j int) bool {
		return semver.Compare(versions[i], versions[j]) < 0
	})
	if ver.String() == utils.NightlyVersionAlias {
		for i := len(versions); i > 0; i-- {
			if utils.Version(versions[i-1]).IsNightly() {
				return utils.Version(versions[i-1]), nil
			}
		}
		return "", errors.Errorf("component(nightly) not installed, please try `tiup install %s:nightly` to install it", component)
	}
	return utils.Version(versions[len(versions)-1]), nil
}

// ComponentInstalledPath returns the path where the component installed
func (p *Profile) ComponentInstalledPath(component string, version utils.Version) (string, error) {
	installedVersion, err := p.GetComponentInstalledVersion(component, version)
	if err != nil {
		return "", err
	}
	return filepath.Join(p.Path(ComponentParentDir), component, installedVersion.String()), nil
}

// SaveTo saves file to the profile directory, path is relative to the
// profile directory of current user
func (p *Profile) saveTo(path string, data []byte, perm os.FileMode) error {
	fullPath := filepath.Join(p.root, path)
	// create sub directory if needed
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return errors.Trace(err)
	}
	return os.WriteFile(fullPath, data, perm)
}

// WriteJSON writes struct to a file (in the profile directory) in JSON format
func (p *Profile) writeJSON(path string, data any) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return errors.Trace(err)
	}
	return p.saveTo(path, jsonData, 0644)
}

// readJSON read file and unmarshal to target `data`
func (p *Profile) readJSON(path string, data any) error {
	fullPath := filepath.Join(p.root, path)
	file, err := os.Open(fullPath)
	if err != nil {
		return errors.Trace(err)
	}
	defer file.Close()

	return json.NewDecoder(file).Decode(data)
}

// ReadMetaFile reads a Process object from dirName/MetaFilename. Returns (nil, nil) if a metafile does not exist.
func (p *Profile) ReadMetaFile(dirName string) (*Process, error) {
	metaFile := filepath.Join(DataParentDir, dirName, MetaFilename)

	// If the path doesn't contain the meta file, which means startup interrupted
	if utils.IsNotExist(p.Path(metaFile)) {
		return nil, nil
	}

	var process Process
	err := p.readJSON(metaFile, &process)
	return &process, err
}

// InstalledComponents returns the installed components
func (p *Profile) InstalledComponents() ([]string, error) {
	compDir := filepath.Join(p.root, ComponentParentDir, p.name)
	fileInfos, err := os.ReadDir(compDir)
	if err != nil && os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.Trace(err)
	}
	var components []string
	for _, fi := range fileInfos {
		if !fi.IsDir() {
			continue
		}
		components = append(components, fi.Name())
	}
	sort.Strings(components)
	return components, nil
}

// InstalledVersions returns the installed versions of specific component
func (p *Profile) InstalledVersions(component string) ([]string, error) {
	path := filepath.Join(p.root, ComponentParentDir, p.name, component)
	if utils.IsNotExist(path) {
		return nil, nil
	}

	fileInfos, err := os.ReadDir(path)
	if err != nil {
		return nil, errors.Trace(err)
	}
	var versions []string
	for _, fi := range fileInfos {
		if !fi.IsDir() {
			continue
		}
		sub, err := os.ReadDir(filepath.Join(path, fi.Name()))
		if err != nil || len(sub) < 1 {
			continue
		}
		versions = append(versions, fi.Name())
	}
	return versions, nil
}

// VersionIsInstalled returns true if exactly version of component is installed.
func (p *Profile) VersionIsInstalled(component, version string) (bool, error) {
	installed, err := p.InstalledVersions(component)
	if err != nil {
		return false, err
	}
	for _, v := range installed {
		if v == version {
			return true, nil
		}
	}
	return false, nil
}

// ResetMirror reset root.json and cleanup manifests directory
func (p *Profile) ResetMirror(addr, root string) error {
	// Calculating root.json path
	shaWriter := sha256.New()
	if _, err := io.Copy(shaWriter, strings.NewReader(addr)); err != nil {
		return err
	}
	localRoot := p.Path("bin", fmt.Sprintf("%s.root.json", hex.EncodeToString(shaWriter.Sum(nil))[:16]))

	if root == "" {
		switch {
		case utils.IsExist(localRoot):
			root = localRoot
		case strings.HasSuffix(addr, "/"):
			root = addr + "root.json"
		default:
			root = addr + "/root.json"
		}
	}

	// Fetch root.json
	var wc io.ReadCloser
	if strings.HasPrefix(root, "http") {
		resp, err := http.Get(root)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return errors.Errorf("Fetch remote root.json returns http code %d", resp.StatusCode)
		}
		wc = resp.Body
	} else {
		file, err := os.Open(root)
		if err != nil {
			return err
		}
		wc = file
	}
	defer wc.Close()

	f, err := os.OpenFile(p.Path(TrustedDir, p.name, "root.json"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0664)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, wc); err != nil {
		f.Close()
		return err
	}
	f.Close()

	// Only cache remote mirror
	if strings.HasPrefix(addr, "http") && root != localRoot {
		if strings.HasPrefix(root, "http") {
			fmt.Printf("WARN: adding root certificate via internet: %s\n", root)
			fmt.Printf("You can revoke this by remove %s\n", localRoot)
		}
		_ = utils.Copy(p.Path("bin", "root.json"), localRoot)
	}

	if err := os.RemoveAll(p.Path(ManifestParentDir)); err != nil {
		return err
	}

	p.Config.Mirror = addr
	return p.Config.Flush()
}

// Process represents a process as written to a meta file.
type Process struct {
	Component   string    `json:"component"`
	CreatedTime string    `json:"created_time"`
	Pid         int       `json:"pid"`            // PID of the process
	Exec        string    `json:"exec"`           // Path to the binary
	Args        []string  `json:"args,omitempty"` // Command line arguments
	Env         []string  `json:"env,omitempty"`  // Environment variables
	Dir         string    `json:"dir,omitempty"`  // Data directory
	Cmd         *exec.Cmd `json:"-"`
}
