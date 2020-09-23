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
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/spf13/cobra"
)

func newReloadCmd() *cobra.Command {
	var skipRestart bool
	cmd := &cobra.Command{
		Use:   "reload <cluster-name>",
		Short: "Reload a TiDB cluster's config and restart if needed",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			if err := validRoles(gOpt.Roles); err != nil {
				return err
			}

			clusterName := args[0]
			teleCommand = append(teleCommand, scrubClusterName(clusterName))

			return manager.Reload(clusterName, gOpt, skipRestart)
		},
	}

	cmd.Flags().BoolVar(&gOpt.Force, "force", false, "Force reload without transferring PD leader and ignore any error")
	cmd.Flags().StringSliceVarP(&gOpt.Roles, "role", "R", nil, "Only start specified roles")
	cmd.Flags().StringSliceVarP(&gOpt.Nodes, "node", "N", nil, "Only start specified nodes")
	cmd.Flags().Int64Var(&gOpt.APITimeout, "transfer-timeout", 300, "Timeout in seconds when transferring PD and TiKV store leaders")
	cmd.Flags().BoolVarP(&gOpt.IgnoreConfigCheck, "ignore-config-check", "", false, "Ignore the config check result")
	cmd.Flags().BoolVar(&skipRestart, "skip-restart", false, "Only refresh configuration to remote and do not restart services")

	return cmd
}

func validRoles(roles []string) error {
	for _, r := range roles {
		match := false
		for _, has := range spec.AllComponentNames() {
			if r == has {
				match = true
				break
			}
		}

		if !match {
			return perrs.Errorf("not valid role: %s, should be one of: %v", r, spec.AllComponentNames())
		}
	}

	return nil
}
