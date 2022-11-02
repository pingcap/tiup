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
	"path/filepath"

	"github.com/pingcap/tiup/pkg/cluster/manager"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/spf13/cobra"
)

func newUpgradeCmd() *cobra.Command {
	offlineMode := false
	opt := manager.DeployOptions{
		IdentityFile: filepath.Join(utils.UserHome(), ".ssh", "id_rsa"),
	}
	cmd := &cobra.Command{
		Use:   "upgrade <cluster-name> <version>",
		Short: "Upgrade a specified TiDB cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return cmd.Help()
			}

			clusterName := args[0]
			version, err := utils.FmtVer(args[1])
			if err != nil {
				return err
			}
			clusterReport.ID = scrubClusterName(clusterName)
			teleCommand = append(teleCommand, scrubClusterName(clusterName))
			teleCommand = append(teleCommand, version)

			return cm.Upgrade(clusterName, version, gOpt, skipConfirm, offlineMode, opt)
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
	cmd.Flags().StringVarP(&opt.User, "user", "u", utils.CurrentUser(), "The user name to login via SSH. The user must has root (or sudo) privilege.")
	cmd.Flags().StringVarP(&opt.IdentityFile, "identity_file", "i", opt.IdentityFile, "The path of the SSH identity file. If specified, public key authentication will be used.")
	cmd.Flags().BoolVarP(&opt.UsePassword, "password", "p", false, "Use password of target hosts. If specified, password authentication will be used.")
	cmd.Flags().BoolVar(&gOpt.Force, "force", false, "Force upgrade without transferring PD leader")
	cmd.Flags().Uint64Var(&gOpt.APITimeout, "transfer-timeout", 600, "Timeout in seconds when transferring PD and TiKV store leaders, also for TiCDC drain one capture")
	cmd.Flags().BoolVarP(&gOpt.IgnoreConfigCheck, "ignore-config-check", "", false, "Ignore the config check result")
	cmd.Flags().BoolVarP(&offlineMode, "offline", "", false, "Upgrade a stopped cluster")

	return cmd
}
