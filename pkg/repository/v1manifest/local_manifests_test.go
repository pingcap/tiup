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

package v1manifest

import (
	"os"
	"path"
	"testing"

	"github.com/google/uuid"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/stretchr/testify/assert"
)

// Create a profile directory
// Contained stuff:
//   - index.json: with wrong signature
//   - snapshot.json: correct
//   - tidb.json: correct
//   - timestamp: with expired timestamp
func genPollutedProfileDir() (string, error) {
	uid := uuid.New().String()
	dir := path.Join("/tmp", uid)

	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	if err := utils.Copy(path.Join(wd, "testdata", "polluted"), dir); err != nil {
		return "", err
	}

	return dir, nil
}

func TestPollutedManifest(t *testing.T) {
	profileDir, err := genPollutedProfileDir()
	assert.Nil(t, err)
	defer os.RemoveAll(profileDir)

	profile := localdata.NewProfile(profileDir, "todo", &localdata.TiUPConfig{})
	manifest, err := NewManifests(profile)
	assert.Nil(t, err)

	index := Index{}
	_, exist, err := manifest.LoadManifest(&index)
	assert.Nil(t, err)
	assert.False(t, exist)

	snap := Snapshot{}
	_, exist, err = manifest.LoadManifest(&snap)
	assert.Nil(t, err)
	assert.True(t, exist)

	timestamp := Timestamp{}
	_, exist, err = manifest.LoadManifest(&timestamp)
	assert.Nil(t, err)
	assert.False(t, exist)

	filename := ComponentManifestFilename("tidb")
	tidb, err := manifest.LoadComponentManifest(&ComponentItem{
		Owner: "pingcap",
		URL:   "/tidb.json",
	}, filename)
	assert.NotNil(t, err) // Because index.json not load successfully
	assert.Nil(t, tidb)
}
