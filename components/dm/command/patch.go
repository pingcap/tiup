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
	perrs "github.com/pingcap/errors"
	"github.com/spf13/cobra"
)

func newPatchCmd() *cobra.Command {
	var (
		overwrite bool
	)
	cmd := &cobra.Command{
		Use:   "patch <cluster-name> <package-path>",
		Short: "Replace the remote package with a specified package and restart the service",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return cmd.Help()
			}

			if err := validRoles(gOpt.Roles); err != nil {
				return err
			}

			if len(gOpt.Nodes) == 0 && len(gOpt.Roles) == 0 {
				return perrs.New("the flag -R or -N must be specified at least one")
			}

			clusterName := args[0]

			return manager.Patch(clusterName, args[1], gOpt, overwrite)
		},
	}

	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "Use this package in the future scale-out operations")
	cmd.Flags().StringSliceVarP(&gOpt.Nodes, "node", "N", nil, "Specify the nodes")
	cmd.Flags().StringSliceVarP(&gOpt.Roles, "role", "R", nil, "Specify the role")
	cmd.Flags().Int64Var(&gOpt.APITimeout, "transfer-timeout", 300, "Timeout in seconds when transferring dm-master leaders")
	return cmd
}
