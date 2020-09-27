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
	"context"
	"io/ioutil"
	"path"

	"github.com/pingcap/tiup/pkg/cliutil"
	"github.com/pingcap/tiup/pkg/cluster"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/report"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/errutil"
	telemetry2 "github.com/pingcap/tiup/pkg/telemetry"
	"github.com/pingcap/tiup/pkg/utils"
	tiuputils "github.com/pingcap/tiup/pkg/utils"
	"github.com/spf13/cobra"
)

var (
	teleReport    *telemetry2.Report
	clusterReport *telemetry2.ClusterReport
	teleNodeInfos []*telemetry2.NodeInfo
	teleTopology  string
	teleCommand   []string
)

var (
	errNSDeploy            = errNS.NewSubNamespace("deploy")
	errDeployNameDuplicate = errNSDeploy.NewType("name_dup", errutil.ErrTraitPreCheck)
)

func newDeploy() *cobra.Command {
	opt := cluster.DeployOptions{
		IdentityFile: path.Join(tiuputils.UserHome(), ".ssh", "id_rsa"),
	}
	cmd := &cobra.Command{
		Use:          "deploy <cluster-name> <version> <topology.yaml>",
		Short:        "Deploy a cluster for production",
		Long:         "Deploy a cluster for production. SSH connection will be used to deploy files, as well as creating system users for running the service.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			shouldContinue, err := cliutil.CheckCommandArgsAndMayPrintHelp(cmd, args, 3)
			if err != nil {
				return err
			}
			if !shouldContinue {
				return nil
			}

			// natvie ssh has it's own logic to find the default identity_file
			if gOpt.SSHType == executor.SSHTypeSystem && !utils.IsFlagSetByUser(cmd.Flags(), "identity_file") {
				opt.IdentityFile = ""
			}

			clusterName := args[0]
			version := args[1]
			teleCommand = append(teleCommand, scrubClusterName(clusterName))
			teleCommand = append(teleCommand, version)

			topoFile := args[2]
			if data, err := ioutil.ReadFile(topoFile); err == nil {
				teleTopology = string(data)
			}

			return manager.Deploy(
				clusterName,
				version,
				topoFile,
				opt,
				postDeployHook,
				skipConfirm,
				gOpt.OptTimeout,
				gOpt.SSHTimeout,
				gOpt.SSHType,
			)
		},
	}

	cmd.Flags().StringVarP(&opt.User, "user", "u", tiuputils.CurrentUser(), "The user name to login via SSH. The user must has root (or sudo) privilege.")
	cmd.Flags().BoolVarP(&opt.SkipCreateUser, "skip-create-user", "", false, "Skip creating the user specified in topology (experimental).")
	cmd.Flags().StringVarP(&opt.IdentityFile, "identity_file", "i", opt.IdentityFile, "The path of the SSH identity file. If specified, public key authentication will be used.")
	cmd.Flags().BoolVarP(&opt.UsePassword, "password", "p", false, "Use password of target hosts. If specified, password authentication will be used.")
	cmd.Flags().BoolVarP(&opt.IgnoreConfigCheck, "ignore-config-check", "", opt.IgnoreConfigCheck, "Ignore the config check result")

	return cmd
}

func postDeployHook(builder *task.Builder, topo spec.Topology) {
	nodeInfoTask := task.NewBuilder().Func("Check status", func(ctx *task.Context) error {
		var err error
		teleNodeInfos, err = operator.GetNodeInfo(context.Background(), ctx, topo)
		_ = err
		// intend to never return error
		return nil
	}).BuildAsStep("Check status").SetHidden(true)

	if report.Enable() {
		builder.ParallelStep("+ Check status", false, nodeInfoTask)
	}

	enableTask := task.NewBuilder().Func("Enable cluster", func(ctx *task.Context) error {
		return operator.Enable(ctx, topo, operator.Options{}, true)
	}).BuildAsStep("Enable cluster").SetHidden(true)

	builder.ParallelStep("+ Enable cluster", false, enableTask)
}
