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
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/stretchr/testify/require"
)

func TestResetMirror(t *testing.T) {
	uuid := uuid.New().String()
	root := path.Join("/tmp", uuid)
	_ = os.Mkdir(root, 0o755)
	_ = os.Mkdir(path.Join(root, "bin"), 0o755)
	t.Cleanup(func() {
		os.RemoveAll(root)
	})

	cfg, _ := InitConfig(root)
	profile := NewProfile(root, cfg)

	require.NoError(t, profile.ResetMirror("https://tiup-mirrors.pingcap.com", ""))
	require.Error(t, profile.ResetMirror("https://example.com", ""))
	require.NoError(t, profile.ResetMirror("https://example.com", "https://tiup-mirrors.pingcap.com/root.json"))

	require.NoError(t, utils.Copy(path.Join(root, "bin"), path.Join(root, "mock-mirror")))

	require.NoError(t, profile.ResetMirror(path.Join(root, "mock-mirror"), ""))
	require.Error(t, profile.ResetMirror(root, ""))
	require.NoError(t, profile.ResetMirror(root, path.Join(root, "mock-mirror", "root.json")))
}

func TestWriteMetaFile_abs(t *testing.T) {
	tmpdir, err := os.MkdirTemp("/tmp", "tiup_test_metafile")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)
	instance := filepath.Join(tmpdir, "testinstance")
	cfg, _ := InitConfig(tmpdir)
	profile := NewProfile(tmpdir, cfg)

	p := Process{}
	err = profile.WriteMetaFile(instance, &p)
	require.NoError(t, err)

	_, err = os.Stat(instance)
	require.NoError(t, err)
}

func TestWriteMetaFile_rel(t *testing.T) {
	tmpdir, err := os.MkdirTemp("/tmp", "tiup_test_metafile")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)
	instance := "testinstance"
	cfg, _ := InitConfig(tmpdir)
	profile := NewProfile(tmpdir, cfg)

	p := Process{}
	err = profile.WriteMetaFile(instance, &p)
	require.NoError(t, err)

	_, err = os.Stat(profile.Path(instance))
	require.NoError(t, err)
}

func TestWriteMetaFile_readback(t *testing.T) {
	tmpdir, err := os.MkdirTemp("/tmp", "tiup_test_metafile")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)
	instance := filepath.Join(tmpdir, DataParentDir, "testinstance")
	cfg, _ := InitConfig(tmpdir)
	profile := NewProfile(tmpdir, cfg)

	p := Process{}
	err = profile.WriteMetaFile(instance, &p)
	require.NoError(t, err)

	p2, err := profile.ReadMetaFile(filepath.Base(instance))
	require.NoError(t, err)
	require.NotNil(t, p2)
	require.Equal(t, p, *p2)
}
