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

package spec

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

type TestMetadata struct {
	BaseMeta
	Topo *TestTopology
}

func (m *TestMetadata) SetVersion(s string) {
	m.BaseMeta.Version = s
}

func (m *TestMetadata) SetUser(s string) {
	m.BaseMeta.User = s
}

func (m *TestMetadata) GetTopology() Topology {
	return m.Topo
}

func (m *TestMetadata) GetBaseMeta() *BaseMeta {
	return &m.BaseMeta
}

func (t *TestTopology) Merge(topo Topology) Topology {
	panic("not support")
}

func (t *TestTopology) FillHostArchOrOS(hostArchOrOS map[string]string, fullType FullHostType) error {
	panic("not support")
}

func (m *TestMetadata) SetTopology(topo Topology) {
	testTopo, ok := topo.(*TestTopology)
	if !ok {
		panic("wrong toplogy type")
	}

	m.Topo = testTopo
}

type TestTopology struct {
	base BaseTopo
}

func (t *TestTopology) Validate() error {
	return nil
}

func (t *TestTopology) TLSConfig(dir string) (*tls.Config, error) {
	return nil, nil
}

func (t *TestTopology) NewPart() Topology {
	panic("not support")
}

func (t *TestTopology) MergeTopo(topo Topology) Topology {
	panic("not support")
}

func (t *TestTopology) Type() string {
	return TopoTypeTiDB
}

func (t *TestTopology) BaseTopo() *BaseTopo {
	return &t.base
}

func (t *TestTopology) ComponentsByStartOrder() []Component {
	return nil
}

func (t *TestTopology) ComponentsByStopOrder() []Component {
	return nil
}

func (t *TestTopology) ComponentsByUpdateOrder() []Component {
	return nil
}

func (t *TestTopology) IterInstance(fn func(instance Instance)) {
}

func (t *TestTopology) GetMonitoredOptions() *MonitoredOptions {
	return nil
}

func (t *TestTopology) GetGlobalOptions() GlobalOptions {
	return GlobalOptions{}
}

func (t *TestTopology) CountDir(host string, dir string) int {
	return 0
}

func (t *TestTopology) GetGrafanaConfig() map[string]string {
	return nil
}
func TestSpec(t *testing.T) {
	dir, err := os.MkdirTemp("", "test-*")
	assert.Nil(t, err)

	spec := NewSpec(dir, func() Metadata {
		return new(TestMetadata)
	})
	names, err := spec.List()
	assert.Nil(t, err)
	assert.Len(t, names, 0)

	// Should ignore directory without meta file.
	err = os.Mkdir(filepath.Join(dir, "dummy"), 0755)
	assert.Nil(t, err)
	names, err = spec.List()
	assert.Nil(t, err)
	assert.Len(t, names, 0)

	exist, err := spec.Exist("dummy")
	assert.Nil(t, err)
	assert.False(t, exist)

	var meta1 = &TestMetadata{
		BaseMeta: BaseMeta{
			Version: "1.1.1",
		},
		Topo: &TestTopology{},
	}
	var meta2 = &TestMetadata{
		BaseMeta: BaseMeta{
			Version: "2.2.2",
		},
		Topo: &TestTopology{},
	}

	err = spec.SaveMeta("name1", meta1)
	assert.Nil(t, err)

	err = spec.SaveMeta("name2", meta2)
	assert.Nil(t, err)

	getMeta := new(TestMetadata)
	err = spec.Metadata("name1", getMeta)
	assert.Nil(t, err)
	assert.Equal(t, meta1, getMeta)

	err = spec.Metadata("name2", getMeta)
	assert.Nil(t, err)
	assert.Equal(t, meta2, getMeta)

	names, err = spec.List()
	assert.Nil(t, err)
	assert.Len(t, names, 2)
	sort.Strings(names)
	assert.Equal(t, "name1", names[0])
	assert.Equal(t, "name2", names[1])

	exist, err = spec.Exist("name1")
	assert.Nil(t, err)
	assert.True(t, exist)

	exist, err = spec.Exist("name2")
	assert.Nil(t, err)
	assert.True(t, exist)

	specList, err := spec.GetAllClusters()
	assert.Nil(t, err)
	assert.Equal(t, meta1, specList["name1"])
	assert.Equal(t, meta2, specList["name2"])

	// remove name1 and check again.
	err = spec.Remove("name1")
	assert.Nil(t, err)
	exist, err = spec.Exist("name1")
	assert.Nil(t, err)
	assert.False(t, exist)

	// remove a not exist cluster should be fine
	err = spec.Remove("name1")
	assert.Nil(t, err)
}
