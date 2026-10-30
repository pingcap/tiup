// Copyright 2026 PingCAP, Inc.
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

package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestLocalDiagRedactPath(t *testing.T) {
	dir := t.TempDir()
	driver := filepath.Join(dir, "tiup")
	require.NoError(t, os.WriteFile(driver, []byte("test driver"), 0700))
	_, err := localDiagRedactPath(driver)
	require.ErrorContains(t, err, "not found beside TiUP")
	binary := filepath.Join(dir, "diag-redact")
	require.NoError(t, os.WriteFile(binary, []byte("test component"), 0600))
	_, err = localDiagRedactPath(driver)
	require.ErrorContains(t, err, "not a regular executable")
	require.NoError(t, os.Chmod(binary, 0700))
	want, err := filepath.EvalSymlinks(binary)
	require.NoError(t, err)
	got, err := localDiagRedactPath(driver)
	require.NoError(t, err)
	require.Equal(t, want, got)
	link := filepath.Join(t.TempDir(), "tiup")
	require.NoError(t, os.Symlink(driver, link))
	got, err = localDiagRedactPath(link)
	require.NoError(t, err)
	require.Equal(t, want, got)
	require.NoError(t, os.Remove(binary))
	require.NoError(t, os.Mkdir(binary, 0700))
	_, err = localDiagRedactPath(driver)
	require.ErrorContains(t, err, "not a regular executable")
}

func TestLocalDiagRedactSkipsOnlyItsOwnMirrorInitialization(t *testing.T) {
	for _, args := range [][]string{nil, {"collect", "cluster"}, {"--help"}} {
		require.True(t, shouldSkipEnvInit(newDiagRedactCmd(), args))
	}
	require.False(t, shouldSkipEnvInit(&cobra.Command{Use: "tiup"}, []string{"diag"}))
	require.False(t, shouldSkipEnvInit(&cobra.Command{Use: "install"}, []string{"diag"}))
}
