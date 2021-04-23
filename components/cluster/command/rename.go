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

package command

import (
	"github.com/spf13/cobra"
)

func newRenameCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rename <old-cluster-name> <new-cluster-name>",
		Short: "Rename the cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return cmd.Help()
			}

			if err := validRoles(gOpt.Roles); err != nil {
				return err
			}

			oldClusterName := args[0]
			newClusterName := args[1]
			clusterReport.ID = scrubClusterName(oldClusterName)
			teleCommand = append(teleCommand, scrubClusterName(oldClusterName))

			return cm.Rename(oldClusterName, gOpt, newClusterName, skipConfirm)
		},
	}

	return cmd
}
