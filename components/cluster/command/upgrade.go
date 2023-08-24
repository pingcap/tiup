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
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/spf13/cobra"
)

func newUpgradeCmd() *cobra.Command {
	offlineMode := false
	var tidbVer, tikvVer, pdVer, tiflashVer, kvcdcVer, dashboardVer, cdcVer, alertmanagerVer, nodeExporterVer, blackboxExporterVer string

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

			componentVersions := map[string]string{
				spec.ComponentDashboard:        dashboardVer,
				spec.ComponentAlertmanager:     alertmanagerVer,
				spec.ComponentTiDB:             tidbVer,
				spec.ComponentTiKV:             tikvVer,
				spec.ComponentPD:               pdVer,
				spec.ComponentTiFlash:          tiflashVer,
				spec.ComponentTiKVCDC:          kvcdcVer,
				spec.ComponentCDC:              cdcVer,
				spec.ComponentBlackboxExporter: blackboxExporterVer,
				spec.ComponentNodeExporter:     nodeExporterVer,
			}

			return cm.Upgrade(clusterName, version, componentVersions, gOpt, skipConfirm, offlineMode)
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
	cmd.Flags().BoolVar(&gOpt.Force, "force", false, "Force upgrade without transferring PD leader")
	cmd.Flags().Uint64Var(&gOpt.APITimeout, "transfer-timeout", 600, "Timeout in seconds when transferring PD and TiKV store leaders, also for TiCDC drain one capture")
	cmd.Flags().BoolVarP(&gOpt.IgnoreConfigCheck, "ignore-config-check", "", false, "Ignore the config check result")
	cmd.Flags().BoolVarP(&offlineMode, "offline", "", false, "Upgrade a stopped cluster")
	cmd.Flags().StringVar(&gOpt.SSHCustomScripts.BeforeRestartInstance.Raw, "pre-upgrade-script", "", "(EXPERIMENTAL) Custom script to be executed on each server before the server is upgraded")
	cmd.Flags().StringVar(&gOpt.SSHCustomScripts.AfterRestartInstance.Raw, "post-upgrade-script", "", "(EXPERIMENTAL) Custom script to be executed on each server after the server is upgraded")

	cmd.Flags().StringVar(&tidbVer, "tidb-version", "", "Specify the version of tidb to upgrade to")
	cmd.Flags().StringVar(&tikvVer, "tikv-version", "", "Specify the version of tikv to upgrade to")
	cmd.Flags().StringVar(&pdVer, "pd-version", "", "Specify the version of pd to upgrade to")
	cmd.Flags().StringVar(&tiflashVer, "tiflash-version", "", "Specify the version of tiflash to upgrade to")
	cmd.Flags().StringVar(&dashboardVer, "tidb-dashboard-version", "", "Specify the version of tidb-dashboard to upgrade to")
	cmd.Flags().StringVar(&cdcVer, "cdc-version", "", "Specify the version of cdc to upgrade to")
	cmd.Flags().StringVar(&kvcdcVer, "tikv-cdc-version", "", "Specify the cersion of tikc-cdc to upgrade to")
	cmd.Flags().StringVar(&alertmanagerVer, "alertmanager-version", "", "Specify the version of alertmanager to upgrade to")
	cmd.Flags().StringVar(&nodeExporterVer, "node-exporter-version", "", "Specify the version of node-exporter to upgrade to")
	cmd.Flags().StringVar(&blackboxExporterVer, "blackbox-exporter-version", "", "Specify the version of blackbox-exporter to upgrade to")
	return cmd
}
