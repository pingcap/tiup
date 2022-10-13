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
	"encoding/json"
	"io"
	"testing"

	"github.com/pingcap/tiup/pkg/repository/model"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/pingcap/tiup/pkg/utils/mock"
	"github.com/stretchr/testify/assert"
)

func manifest2str(m *v1manifest.Manifest) string {
	b, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func baseMirror4test(ownerKeys map[string]*v1manifest.KeyInfo) Mirror {
	return &MockMirror{
		Resources: map[string]string{
			"snapshot.json": manifest2str(&v1manifest.Manifest{
				Signed: &v1manifest.Snapshot{
					Meta: map[string]v1manifest.FileVersion{
						"/index.json": {
							Version: 1,
						},
						"/test.json": {
							Version: 1,
						},
					},
				},
			}),
			"1.index.json": manifest2str(&v1manifest.Manifest{
				Signed: &v1manifest.Index{
					Owners: map[string]v1manifest.Owner{
						"pingcap": {
							Name:      "PingCAP",
							Keys:      ownerKeys,
							Threshold: 1,
						},
					},
					Components: map[string]v1manifest.ComponentItem{
						"test": {
							Owner: "pingcap",
							URL:   "/test.json",
						},
					},
				},
			}),
			"1.test.json": manifest2str(&v1manifest.Manifest{
				Signed: &v1manifest.Component{
					Platforms: map[string]map[string]v1manifest.VersionItem{
						"linux/amd64": {
							"v1.0.0": {
								URL:   "/test-v1.0.0-linux-amd64.tar.gz",
								Entry: "test",
							},
						},
					},
				},
			}),
		},
	}
}

func sourceMirror4test() Mirror {
	return &MockMirror{
		Resources: map[string]string{
			"snapshot.json": manifest2str(&v1manifest.Manifest{
				Signed: &v1manifest.Snapshot{
					Meta: map[string]v1manifest.FileVersion{
						"/index.json": {
							Version: 2,
						},
						"/hello.json": {
							Version: 1,
						},
						"/test.json": {
							Version: 1,
						},
					},
				},
			}),
			"2.index.json": manifest2str(&v1manifest.Manifest{
				Signed: &v1manifest.Index{
					Components: map[string]v1manifest.ComponentItem{
						"test": {
							Owner: "pingcap",
							URL:   "/test.json",
						},
						"hello": {
							Owner: "pingcap",
							URL:   "/hello.json",
						},
					},
				},
			}),
			"1.test.json": manifest2str(&v1manifest.Manifest{
				Signed: &v1manifest.Component{
					Platforms: map[string]map[string]v1manifest.VersionItem{
						"linux/amd64": {
							"v1.0.0": {
								URL:   "/test-v1.0.0-linux-amd64.tar.gz",
								Entry: "test",
							},
							"v1.0.1": {
								URL:   "/test-v1.0.1-linux-amd64.tar.gz",
								Entry: "test",
							},
						},
						"linux/arm64": {
							"v1.0.0": {
								URL:   "/test-v1.0.0-linux-arm64.tar.gz",
								Entry: "test",
							},
						},
					},
				},
			}),
			"1.hello.json": manifest2str(&v1manifest.Manifest{
				Signed: &v1manifest.Component{
					Platforms: map[string]map[string]v1manifest.VersionItem{
						"linux/amd64": {
							"v1.0.0": {
								URL:   "/hello-v1.0.0-linux-amd64.tar.gz",
								Entry: "hello",
							},
						},
					},
				},
			}),
			"hello-v1.0.0-linux-amd64.tar.gz": "hello-v1.0.0-linux-amd64.tar.gz",
			"test-v1.0.1-linux-amd64.tar.gz":  "test-v1.0.1-linux-amd64.tar.gz",
			"test-v1.0.0-linux-arm64.tar.gz":  "test-v1.0.0-linux-arm64.tar.gz",
		},
	}
}

func TestDiffMirror(t *testing.T) {
	base := baseMirror4test(nil)
	source := sourceMirror4test()

	items, err := diffMirror(base, source)
	assert.Nil(t, err)
	assert.Equal(t, 3, len(items))
	for _, it := range items {
		assert.Contains(t, []string{
			"/hello-v1.0.0-linux-amd64.tar.gz",
			"/test-v1.0.0-linux-arm64.tar.gz",
			"/test-v1.0.1-linux-amd64.tar.gz",
		}, it.versionItem.URL)
	}
}

func TestMergeMirror(t *testing.T) {
	ki, err := v1manifest.GenKeyInfo()
	if err != nil {
		panic(err)
	}
	id, err := ki.ID()
	if err != nil {
		panic(err)
	}

	keys := map[string]*v1manifest.KeyInfo{
		id: ki,
	}
	base := baseMirror4test(keys)
	source := sourceMirror4test()

	// manifestList := []*v1manifest.Manifest{}
	// componentInfoList := []model.ComponentInfo{}

	err = MergeMirror(keys, base, source)
	assert.Nil(t, err)

	mock.With("Publish", func(manifest *v1manifest.Manifest, info model.ComponentInfo) {
		assert.Contains(t, []string{
			"hello-v1.0.0-linux-amd64.tar.gz",
			"test-v1.0.0-linux-arm64.tar.gz",
			"test-v1.0.1-linux-amd64.tar.gz",
		}, info.Filename())

		b, err := io.ReadAll(info)
		assert.Nil(t, err)
		assert.Contains(t, []string{
			"hello-v1.0.0-linux-amd64.tar.gz",
			"test-v1.0.0-linux-arm64.tar.gz",
			"test-v1.0.1-linux-amd64.tar.gz",
		}, string(b))
	})()
}

func TestFetchIndex(t *testing.T) {
	source := sourceMirror4test()
	index, err := fetchIndexManifestFromMirror(source)
	assert.Nil(t, err)
	assert.NotEmpty(t, index.Components["hello"].URL)
	assert.NotEmpty(t, index.Components["test"].URL)

	base := baseMirror4test(nil)
	index, err = fetchIndexManifestFromMirror(base)
	assert.Nil(t, err)
	assert.NotEmpty(t, index.Owners["pingcap"].Name)
}

func TestFetchComponent(t *testing.T) {
	source := sourceMirror4test()
	comp, err := fetchComponentManifestFromMirror(source, "test")
	assert.Nil(t, err)
	assert.NotEmpty(t, comp.Platforms["linux/amd64"])
	assert.NotEmpty(t, comp.Platforms["linux/arm64"])
	assert.NotEmpty(t, comp.Platforms["linux/amd64"]["v1.0.0"].URL)
	assert.NotEmpty(t, comp.Platforms["linux/amd64"]["v1.0.1"].URL)
	assert.NotEmpty(t, comp.Platforms["linux/arm64"]["v1.0.0"].URL)
}
