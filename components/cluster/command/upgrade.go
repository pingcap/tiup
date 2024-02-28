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
	ignoreVersionCheck := false
	var tidbVer, tikvVer, pdVer, tsoVer, schedulingVer, tiflashVer, kvcdcVer, dashboardVer, cdcVer, alertmanagerVer, nodeExporterVer, blackboxExporterVer, tiproxyVer string

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
				spec.ComponentTSO:              tsoVer,
				spec.ComponentScheduling:       schedulingVer,
				spec.ComponentTiFlash:          tiflashVer,
				spec.ComponentTiKVCDC:          kvcdcVer,
				spec.ComponentCDC:              cdcVer,
				spec.ComponentTiProxy:          tiproxyVer,
				spec.ComponentBlackboxExporter: blackboxExporterVer,
				spec.ComponentNodeExporter:     nodeExporterVer,
			}

			return cm.Upgrade(clusterName, version, componentVersions, gOpt, skipConfirm, offlineMode, ignoreVersionCheck)
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
	cmd.Flags().BoolVarP(&ignoreVersionCheck, "ignore-version-check", "", false, "Ignore checking if target version is bigger than current version")
	cmd.Flags().StringVar(&gOpt.SSHCustomScripts.BeforeRestartInstance.Raw, "pre-upgrade-script", "", "(EXPERIMENTAL) Custom script to be executed on each server before the server is upgraded")
	cmd.Flags().StringVar(&gOpt.SSHCustomScripts.AfterRestartInstance.Raw, "post-upgrade-script", "", "(EXPERIMENTAL) Custom script to be executed on each server after the server is upgraded")

	// cmd.Flags().StringVar(&tidbVer, "tidb-version", "", "Fix the version of tidb and no longer follows the cluster version.")
	cmd.Flags().StringVar(&tikvVer, "tikv-version", "", "Fix the version of tikv and no longer follows the cluster version.")
	cmd.Flags().StringVar(&pdVer, "pd-version", "", "Fix the version of pd and no longer follows the cluster version.")
	cmd.Flags().StringVar(&tsoVer, "tso-version", "", "Fix the version of tso and no longer follows the cluster version.")
	cmd.Flags().StringVar(&schedulingVer, "scheduling-version", "", "Fix the version of scheduling and no longer follows the cluster version.")
	cmd.Flags().StringVar(&tiflashVer, "tiflash-version", "", "Fix the version of tiflash and no longer follows the cluster version.")
	cmd.Flags().StringVar(&dashboardVer, "tidb-dashboard-version", "", "Fix the version of tidb-dashboard and no longer follows the cluster version.")
	cmd.Flags().StringVar(&cdcVer, "cdc-version", "", "Fix the version of cdc and no longer follows the cluster version.")
	cmd.Flags().StringVar(&kvcdcVer, "tikv-cdc-version", "", "Fix the version of tikv-cdc and no longer follows the cluster version.")
	cmd.Flags().StringVar(&alertmanagerVer, "alertmanager-version", "", "Fix the version of alertmanager and no longer follows the cluster version.")
	cmd.Flags().StringVar(&nodeExporterVer, "node-exporter-version", "", "Fix the version of node-exporter and no longer follows the cluster version.")
	cmd.Flags().StringVar(&blackboxExporterVer, "blackbox-exporter-version", "", "Fix the version of blackbox-exporter and no longer follows the cluster version.")
	cmd.Flags().StringVar(&tiproxyVer, "tiproxy-version", "", "Fix the version of tiproxy and no longer follows the cluster version.")
	return cmd
}
