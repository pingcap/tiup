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
	"github.com/pingcap-incubator/tiup/pkg/meta"
	"github.com/spf13/cobra"
)

func newRepoAddCompCmd(env *meta.Environment) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <component-id> <platform> <version> <file>",
		Short: "Add a file to a component",
		Long:  `Add a file to a component, and set its metadata of platform ID and version.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 4 {
				cmd.Help()
				return nil
			}

			return addCompFile(repoPath, args[0], args[1], args[2], args[3])
		},
	}

	return cmd
}

func addCompFile(repo, id, platform, version, file string) error {
	// TODO
	return nil
}
