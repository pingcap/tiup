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

package profile

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"

	"github.com/c4pt0r/tiup/pkg/utils"
	"github.com/pingcap/errors"
)

const profileDirName = ".tiup"

// Path returns a full path which is joined with profile directory
func Path(relpath string) (string, error) {
	profileDir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(profileDir, relpath), nil
}

// SaveTo saves file to the profile directory, path is relative to the
// profile directory of current user
func SaveTo(path string, data []byte, perm os.FileMode) error {
	profilePath, err := Dir()
	if err != nil {
		return err
	}
	filePath := filepath.Join(profilePath, path)
	// create sub directory if needed
	if err := utils.CreateDir(filepath.Dir(filePath)); err != nil {
		return err
	}
	return ioutil.WriteFile(filePath, data, perm)
}

// WriteJSON writes struct to a file (in the profile directory) in JSON format
func WriteJSON(path string, data interface{}) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return SaveTo(path, jsonData, 0644)
}

// ReadJSON read file and unmarshal to target `data`
func ReadJSON(path string, data interface{}) error {
	fullPath, err := Path(path)
	if err != nil {
		return err
	}

	file, err := os.Open(fullPath)
	if err != nil {
		return errors.Trace(err)
	}

	return json.NewDecoder(file).Decode(data)
}

// ReadFile reads data from a file in the profile directory
func ReadFile(path string) ([]byte, error) {
	profilePath, err := Dir()
	if err != nil {
		return nil, err
	}

	filePath := path
	if !strings.HasPrefix(filePath, profilePath) {
		filePath = filepath.Join(profilePath, path)
	}
	return ioutil.ReadFile(filePath)
}

// getHomeDir get the home directory of current user (if they have one).
// The result path might be empty.
func getHomeDir() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	return u.HomeDir, nil
}

// MustDir return profile directory path and panic if cannot get
func MustDir() string {
	dir, err := Dir()
	if err != nil {
		panic(err)
	}
	return dir
}

// Dir check for profile directory for the current user, and create
// the directory if not already exist.
// Error may be returned if the directory is not exist and fail to create.
func Dir() (string, error) {
	homeDir, err := getHomeDir()
	if err != nil {
		return "", err
	}
	if len(homeDir) < 1 {
		return "", fmt.Errorf("unable to get home directoy of current user")
	}

	profilePath := path.Join(homeDir, profileDirName)
	if err := utils.CreateDir(profilePath); err != nil {
		return "", err
	}
	return profilePath, nil
}
