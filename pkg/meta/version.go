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
	"encoding/json"
	"os"
	"strings"

	"github.com/c4pt0r/tiup/pkg/utils"
)

const (
	versionFileName = "version.json"
)

var (
	availableChannels = []string{
		"stable",
		"beta",
		//"nightly",
	}
)

// CurrVer stores user defined current update channel and default version
type CurrVer struct {
	Chan string `json:"channel,omitempty"` // update channel
	Ver  string `json:"version,omitempty"` // version
}

// valid checks if the CurrVer object is not fully filled with valid data
func (v *CurrVer) valid() bool {
	if *v == (CurrVer{}) ||
		v.Chan == "" ||
		v.Ver == "" {
		return false
	}
	if semVer, err := utils.FmtVer(v.Ver); err != nil || v.Ver != semVer {
		return false
	}
	return true
}

// GetCurrentVer tries to read current channel and version, and fallback to use
// the latest stable version in component list on failure
func GetCurrentVer() (string, string, error) {
	currVer, err := ReadVersionFile()
	if err != nil && !os.IsNotExist(err) {
		return "", "", err
	}

	if os.IsNotExist(err) || !currVer.valid() {
		// current version not set or invalid, use default (stable latest)
		compMeta, err := ReadComponentList()
		if err != nil {
			return "", "", err
		}
		return "stable", compMeta.Stable, nil
	}
	return currVer.Chan, currVer.Ver, nil
}

// ReadVersionFile reads current version from disk
func ReadVersionFile() (*CurrVer, error) {
	data, err := utils.ReadFile(versionFileName)
	if err != nil {
		return nil, err
	}

	return unmarshalCurrVer(data)
}

// SaveCurrentVersion saves new current version data to disk
func SaveCurrentVersion(currVer *CurrVer) error {
	return utils.WriteJSON(versionFileName, currVer)
}

func unmarshalCurrVer(data []byte) (*CurrVer, error) {
	var ver CurrVer
	if err := json.Unmarshal(data, &ver); err != nil {
		return nil, err
	}
	return &ver, nil
}

// IsValidChannel checks if the input is a valid channel of TiDB components
func IsValidChannel(input string) bool {
	inChan := strings.ToLower(input)
	for _, c := range availableChannels {
		if inChan == strings.ToLower(c) {
			return true
		}
	}
	return false
}
