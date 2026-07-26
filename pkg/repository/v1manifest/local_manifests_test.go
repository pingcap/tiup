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

package v1manifest_test

import (
	"os"
	"testing"

	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/repository/testutil"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/stretchr/testify/assert"
)

func TestPollutedManifest(t *testing.T) {
	fixture, err := testutil.NewPollutedProfileFixture()
	assert.Nil(t, err)
	defer os.RemoveAll(fixture.Dir)

	profile := localdata.NewProfile(fixture.Dir, &localdata.TiUPConfig{})
	manifest, err := v1manifest.NewManifests(profile)
	assert.Nil(t, err)

	index := v1manifest.Index{}
	_, exist, err := manifest.LoadManifest(&index)
	assert.Nil(t, err)
	assert.False(t, exist)

	snap := v1manifest.Snapshot{}
	_, exist, err = manifest.LoadManifest(&snap)
	assert.Nil(t, err)
	assert.True(t, exist)

	timestamp := v1manifest.Timestamp{}
	_, exist, err = manifest.LoadManifest(&timestamp)
	assert.Nil(t, err)
	assert.False(t, exist)

	filename := v1manifest.ComponentManifestFilename("tidb")
	tidb, err := manifest.LoadComponentManifest(&v1manifest.ComponentItem{
		Owner: "pingcap",
		URL:   "/tidb.json",
	}, filename)
	assert.NotNil(t, err) // Because index.json not load successfully
	assert.Nil(t, tidb)
}
