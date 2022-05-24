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
	"os"
	"path"

	"github.com/pingcap/tiup/pkg/cluster/manager"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/telemetry"
	"github.com/pingcap/tiup/pkg/tui"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/spf13/cobra"
)

var (
	errNSDeploy            = errNS.NewSubNamespace("deploy")
	errDeployNameDuplicate = errNSDeploy.NewType("name_dup", utils.ErrTraitPreCheck)
)

func newDeploy() *cobra.Command {
	opt := manager.DeployOptions{
		IdentityFile: path.Join(utils.UserHome(), ".ssh", "id_rsa"),
	}
	cmd := &cobra.Command{
		Use:          "deploy <cluster-name> <version> <topology.yaml>",
		Short:        "Deploy a cluster for production",
		Long:         "Deploy a cluster for production. SSH connection will be used to deploy files, as well as creating system users for running the service.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			shouldContinue, err := tui.CheckCommandArgsAndMayPrintHelp(cmd, args, 3)
			if err != nil {
				return err
			}
			if !shouldContinue {
				return nil
			}

			clusterName := args[0]
			version, err := utils.FmtVer(args[1])
			if err != nil {
				return err
			}
			clusterReport.ID = scrubClusterName(clusterName)
			teleCommand = append(teleCommand, scrubClusterName(clusterName))
			teleCommand = append(teleCommand, version)

			topoFile := args[2]
			if data, err := os.ReadFile(topoFile); err == nil {
				teleTopology = string(data)
			}

			return cm.Deploy(clusterName, version, topoFile, opt, postDeployHook, skipConfirm, gOpt)
		},
	}

	cmd.Flags().StringVarP(&opt.User, "user", "u", utils.CurrentUser(), "The user name to login via SSH. The user must has root (or sudo) privilege.")
	cmd.Flags().BoolVarP(&opt.SkipCreateUser, "skip-create-user", "", false, "(EXPERIMENTAL) Skip creating the user specified in topology.")
	cmd.Flags().StringVarP(&opt.IdentityFile, "identity_file", "i", opt.IdentityFile, "The path of the SSH identity file. If specified, public key authentication will be used.")
	cmd.Flags().BoolVarP(&opt.UsePassword, "password", "p", false, "Use password of target hosts. If specified, password authentication will be used.")
	cmd.Flags().BoolVarP(&gOpt.IgnoreConfigCheck, "ignore-config-check", "", false, "Ignore the config check result of components")
	cmd.Flags().BoolVarP(&opt.NoLabels, "no-labels", "", false, "Don't check TiKV labels")

	return cmd
}

func postDeployHook(builder *task.Builder, topo spec.Topology, gOpt operator.Options) {
	nodeInfoTask := task.NewBuilder(builder.Logger).Func("Check status", func(ctx context.Context) error {
		var err error
		teleNodeInfos, err = operator.GetNodeInfo(ctx, topo)
		_ = err
		// intend to never return error
		return nil
	}).BuildAsStep("Check status").SetHidden(true)

	if telemetry.Enabled() {
		builder.ParallelStep("+ Check status", false, nodeInfoTask)
	}

	enableTask := task.NewBuilder(builder.Logger).Func("Setting service auto start on boot", func(ctx context.Context) error {
		return operator.Enable(ctx, topo, operator.Options{}, true)
	}).BuildAsStep("Enable service").SetHidden(true)

	builder.Parallel(false, enableTask)
}
