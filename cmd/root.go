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

package cmd

import (
	"log"
	"os/user"
	"path/filepath"

	"github.com/c4pt0r/tiup/pkg/localdata"
	"github.com/c4pt0r/tiup/pkg/meta"
	"github.com/spf13/cobra"
)

const profileDirName = ".tiup"

var (
	profile    *localdata.Profile
	rootCmd    *cobra.Command
	repository *meta.Repository
)

func init() {
	rootCmd = &cobra.Command{
		Use:   "tiup",
		Short: "Manifest manager for TiDB",
		Long: `The tiup utility is a command line tool that can help to download
and installing TiDB components to the local system.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	rootCmd.AddCommand(
		newSelfCmd(),
		newComponentCmd(),
		newUpdateCmd(),
		newRunCmd(),
		newShowCmd(),
		newVersionCmd(),
		newCompletionsCmd(),
	)
}

func execute() error {
	u, err := user.Current()
	if err != nil {
		return err
	}

	// Initialize the global profile
	profile = localdata.NewProfile(filepath.Join(u.HomeDir, profileDirName))

	// Initialize the repository
	// Replace the mirror if some subcommands use different mirror address
	mirror := meta.NewMirror(defaultMirror)
	if err := mirror.Open(); err != nil {
		return err
	}
	repository = meta.NewRepository(mirror)
	defer func() { _ = repository.Mirror().Close() }()

	return rootCmd.Execute()
}

// Execute parses the command line argumnts and calls proper functions
func Execute() {
	if err := execute(); err != nil {
		log.Fatal(err)
	}
}
