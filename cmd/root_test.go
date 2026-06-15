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
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestParseRootComponentArgs_ForwardsComponentShortT(t *testing.T) {
	got := parseRootComponentArgs([]string{"dumpling", "--no-views=false", "-T", "test.t11"}, "", false)
	require.Equal(t, rootComponentArgs{
		componentSpec: "dumpling",
		componentArgs: []string{"--no-views=false", "-T", "test.t11"},
	}, got)
}

func TestParseRootComponentArgs_ForwardsComponentTagFlag(t *testing.T) {
	got := parseRootComponentArgs([]string{"playground", "-T", "demo"}, "", false)
	require.Equal(t, rootComponentArgs{
		componentSpec: "playground",
		componentArgs: []string{"-T", "demo"},
	}, got)
}

func TestParseRootComponentArgs_StripsExplicitSeparator(t *testing.T) {
	got := parseRootComponentArgs([]string{"playground", "--", "-T", "demo"}, "/tmp/tidb-server", true)
	require.Equal(t, rootComponentArgs{
		binPath:       "/tmp/tidb-server",
		forcePull:     true,
		componentSpec: "playground",
		componentArgs: []string{"-T", "demo"},
	}, got)
}

func TestShouldSkipEnvInit(t *testing.T) {
	mirrorCmd := &cobra.Command{Use: "mirror"}
	cloneCmd := &cobra.Command{Use: "clone"}
	mirrorCmd.AddCommand(cloneCmd)

	require.True(t, shouldSkipEnvInit(rootCmd, []string{"-h"}))
	require.True(t, shouldSkipEnvInit(rootCmd, []string{"--help"}))
	require.True(t, shouldSkipEnvInit(rootCmd, []string{"-v"}))
	require.False(t, shouldSkipEnvInit(rootCmd, []string{"playground"}))
	require.False(t, shouldSkipEnvInit(cloneCmd, []string{"-h"}))
	require.False(t, shouldSkipEnvInit(cloneCmd, []string{"--help"}))
}
