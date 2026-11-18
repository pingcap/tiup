// Copyright 2026 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package command

import "github.com/spf13/cobra"

func newDetachGrafanaCmd() *cobra.Command {
	var nodes []string
	cmd := &cobra.Command{
		Use:   "detach-grafana <cluster-name>",
		Short: "Transfer Grafana instances out of TiUP management without stopping them",
		Long: "Remove the specified Grafana instances from TiUP metadata without connecting to remote hosts or PD. " +
			"Their processes and files remain in place, and you take responsibility for their lifecycle, " +
			"configuration, dashboards, and monitoring. Finish other TiUP changes before detaching. " +
			"Known directory overlaps are rejected; verify the actual remote layout and external dependencies yourself. " +
			"There is no automatic reattach operation.",
		Example: "  tiup cluster detach-grafana production -N grafana.example.com:3000\n" +
			"  tiup cluster detach-grafana production -N grafana1:3000,grafana2:3000 -y",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cm.DetachGrafana(args[0], nodes, skipConfirm)
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return shellCompGetClusterName(cm, toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
	}
	cmd.Flags().StringSliceVarP(&nodes, "node", "N", nil, "Grafana instance IDs to detach (host:port; required, may be repeated)")
	_ = cmd.MarkFlagRequired("node")
	return cmd
}
