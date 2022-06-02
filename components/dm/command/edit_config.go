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
	"github.com/pingcap/tiup/pkg/cluster/manager"
	"github.com/spf13/cobra"
)

func newEditConfigCmd() *cobra.Command {
	opt := manager.EditConfigOptions{}
	cmd := &cobra.Command{
		Use:   "edit-config <cluster-name>",
		Short: "Edit DM cluster config",
		Long:  "Edit DM cluster config. Will use editor from environment variable `EDITOR`, default use vi",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			clusterName := args[0]

			return cm.EditConfig(clusterName, opt, skipConfirm)
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			switch len(args) {
			case 0:
				return shellCompGetClusterName(cm, toComplete)
			default:
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
		},
	}

	cmd.Flags().StringVarP(&opt.NewTopoFile, "topology-file", "", opt.NewTopoFile, "Use provided topology file to substitute the original one instead of editing it.")

	return cmd
}
