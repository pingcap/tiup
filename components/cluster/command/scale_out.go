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
	"path/filepath"

	"github.com/pingcap/tiup/pkg/cluster"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/report"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/utils"
	tiuputils "github.com/pingcap/tiup/pkg/utils"
	"github.com/spf13/cobra"
)

func newScaleOutCmd() *cobra.Command {
	opt := cluster.ScaleOutOptions{
		IdentityFile: filepath.Join(tiuputils.UserHome(), ".ssh", "id_rsa"),
	}
	cmd := &cobra.Command{
		Use:          "scale-out <cluster-name> <topology.yaml>",
		Short:        "Scale out a TiDB cluster",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return cmd.Help()
			}

			// natvie ssh has it's own logic to find the default identity_file
			if gOpt.SSHType == executor.SSHTypeSystem && !utils.IsFlagSetByUser(cmd.Flags(), "identity_file") {
				opt.IdentityFile = ""
			}

			clusterName := args[0]
			teleCommand = append(teleCommand, scrubClusterName(clusterName))

			topoFile := args[1]
			if data, err := ioutil.ReadFile(topoFile); err == nil {
				teleTopology = string(data)
			}

			return manager.ScaleOut(
				clusterName,
				topoFile,
				postScaleOutHook,
				final,
				opt,
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

	return cmd
}

// Deprecated
func convertStepDisplaysToTasks(t []*task.StepDisplay) []task.Task {
	tasks := make([]task.Task, 0, len(t))
	for _, sd := range t {
		tasks = append(tasks, sd)
	}
	return tasks
}

func final(builder *task.Builder, name string, meta spec.Metadata) {
	builder.UpdateTopology(name,
		tidbSpec.Path(name),
		meta.(*spec.ClusterMeta),
		nil, /* deleteNodeIds */
	)
}

func postScaleOutHook(builder *task.Builder, newPart spec.Topology) {
	nodeInfoTask := task.NewBuilder().Func("Check status", func(ctx *task.Context) error {
		var err error
		teleNodeInfos, err = operator.GetNodeInfo(context.Background(), ctx, newPart)
		_ = err
		// intend to never return error
		return nil
	}).BuildAsStep("Check status").SetHidden(true)

	if report.Enable() {
		builder.Parallel(false, convertStepDisplaysToTasks([]*task.StepDisplay{nodeInfoTask})...)
	}
}
