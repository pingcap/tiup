// Copyright 2021 PingCAP, Inc.
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
	"bytes"
	"fmt"
	"path"
	"text/template"

	"github.com/pingcap/tiup/embed"
	"github.com/spf13/cobra"
)

// TemplateOptions contains the options for print topology template.
type TemplateOptions struct {
	Full  bool // print full template
	Local bool // print and render local template
}

// LocalTemplate contains the variables for print local template.
type LocalTemplate struct {
	GlobalUser          string   // global.user in yaml template
	GlobalGroup         string   // global.group in yaml template
	GlobalSystemdMode   string   // global.systemd_mode in yaml template
	GlobalSSHPort       int      // global.ssh_port in yaml template
	GlobalDeployDir     string   // global.deploy_dir in yaml template
	GlobalDataDir       string   // global.data_dir in yaml template
	GlobalArch          string   // global.arch in yaml template
	MasterServers       []string // master_servers in yaml template
	WorkerServers       []string // worker_servers in yaml template
	MonitoringServers   []string // monitoring_servers in yaml template
	GrafanaServers      []string // grafana_servers in yaml template
	AlertManagerServers []string // alertmanager_servers in yaml template
}

func newTemplateCmd() *cobra.Command {
	opt := TemplateOptions{}
	localOpt := LocalTemplate{}

	cmd := &cobra.Command{
		Use:   "template",
		Short: "Print topology template",
		RunE: func(cmd *cobra.Command, args []string) error {
			name := "minimal.yaml"
			switch {
			case opt.Full:
				name = "topology.example.yaml"
			case opt.Local:
				name = "local.tpl"
			}

			fp := path.Join("examples", "dm", name)
			tpl, err := embed.ReadExample(fp)
			if err != nil {
				return err
			}

			if !opt.Local {
				// print example yaml and return
				fmt.Fprintln(cmd.OutOrStdout(), string(tpl))
				return nil
			}

			// redner template

			// validate arch
			if localOpt.GlobalArch != "amd64" && localOpt.GlobalArch != "arm64" {
				return fmt.Errorf(`supported values are "amd64" or "arm64" in global.arch`)
			}

			// validate number of masters and workers
			if len(localOpt.MasterServers) < 3 {
				return fmt.Errorf(
					"at least 3 masters must be defined (given %d servers)",
					len(localOpt.MasterServers),
				)
			}
			if len(localOpt.WorkerServers) < 3 {
				return fmt.Errorf(
					"at least 3 workers must be defined (given %d servers)",
					len(localOpt.WorkerServers),
				)
			}

			tmpl, err := template.New(name).Parse(string(tpl))
			if err != nil {
				return err
			}
			content := bytes.NewBufferString("")
			if err := tmpl.Execute(content, &localOpt); err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), content.String())
			return nil
		},
	}

	cmd.Flags().BoolVar(&opt.Full, "full", false, "Print the full topology template for DM cluster.")
	cmd.Flags().BoolVar(&opt.Local, "local", false, "Print and render template for deploying a simple DM cluster locally.")

	// template values for rendering
	cmd.Flags().StringVar(&localOpt.GlobalUser, "user", "tidb", "The user who runs the tidb cluster.")
	cmd.Flags().StringVar(&localOpt.GlobalGroup, "group", "", "group is used to specify the group name the user belong to if it's not the same as user.")
	cmd.Flags().StringVar(&localOpt.GlobalSystemdMode, "systemd_mode", "system", "systemd_mode is used to select whether to use sudo permissions.")
	cmd.Flags().IntVar(&localOpt.GlobalSSHPort, "ssh-port", 22, "SSH port of servers in the managed cluster.")
	cmd.Flags().StringVar(&localOpt.GlobalDeployDir, "deploy-dir", "/tidb-deploy", "Storage directory for cluster deployment files, startup scripts, and configuration files.")
	cmd.Flags().StringVar(&localOpt.GlobalDataDir, "data-dir", "/tidb-data", "TiDB Cluster data storage directory.")
	cmd.Flags().StringVar(&localOpt.GlobalArch, "arch", "amd64", "Supported values: \"amd64\", \"arm64\".")
	cmd.Flags().StringSliceVar(&localOpt.MasterServers, "master-servers", []string{"172.19.0.101", "172.19.0.102", "172.19.0.103"}, "List of Master servers")
	cmd.Flags().StringSliceVar(&localOpt.WorkerServers, "worker-servers", []string{"172.19.0.101", "172.19.0.102", "172.19.0.103"}, "List of Worker servers")
	cmd.Flags().StringSliceVar(&localOpt.MonitoringServers, "monitoring-servers", []string{"172.19.0.101"}, "List of monitor servers")
	cmd.Flags().StringSliceVar(&localOpt.GrafanaServers, "grafana-servers", []string{"172.19.0.101"}, "List of grafana servers")
	cmd.Flags().StringSliceVar(&localOpt.AlertManagerServers, "alertmanager-servers", []string{"172.19.0.101"}, "List of alermanager servers")

	return cmd
}
