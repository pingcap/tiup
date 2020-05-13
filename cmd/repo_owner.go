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

func newRepoOwnerCmd(env *meta.Environment) *cobra.Command {
	var (
		repoPath string
	)
	cmd := &cobra.Command{
		Use:   "owner <id> <name>",
		Short: "Create a new owner for the repository",
		Long: `Create a new owner role for the repository, the owner can then perform management
actions on authorized resources.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				cmd.Help()
				return nil
			}

			return createOwner(repoPath, args[0], args[1])
		},
	}

	cmd.Flags().StringVar(&repoPath, "repo", "", "Path to the repository")

	return cmd
}

func createOwner(repo, id, name string) error {
	// TODO
	return nil
}
