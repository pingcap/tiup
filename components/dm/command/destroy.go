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

/*
import (
	"errors"
	"os"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cliutil"
	"github.com/pingcap/tiup/pkg/cluster/meta"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/logger"
	"github.com/pingcap/tiup/pkg/logger/log"
	tiuputils "github.com/pingcap/tiup/pkg/utils"
	"github.com/spf13/cobra"
)

func newDestroyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "destroy <cluster-name>",
		Short: "Destroy a specified DM cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			clusterName := args[0]
			if tiuputils.IsNotExist(meta.ClusterPath(clusterName, meta.MetaFileName)) {
				return perrs.Errorf("cannot destroy non-exists cluster %s", clusterName)
			}

			logger.EnableAuditLog()
			metadata, err := meta.DMMetadata(clusterName)
			if err != nil && !errors.Is(perrs.Cause(err), meta.ValidateErr) {
				return err
			}

			if !skipConfirm {
				if err := cliutil.PromptForConfirmOrAbortError(
					"This operation will destroy DM %s cluster %s and its data.\nDo you want to continue? [y/N]:",
					color.HiYellowString(metadata.Version),
					color.HiYellowString(clusterName)); err != nil {
					return err
				}
				log.Infof("Destroying cluster...")
			}

			t := task.NewBuilder().
				SSHKeySet(
					meta.ClusterPath(clusterName, "ssh", "id_rsa"),
					meta.ClusterPath(clusterName, "ssh", "id_rsa.pub")).
				ClusterSSH(metadata.Topology, metadata.User, gOpt.SSHTimeout).
				ClusterOperate(metadata.Topology, operator.StopOperation, operator.Options{}).
				ClusterOperate(metadata.Topology, operator.DestroyOperation, operator.Options{}).
				Build()

			if err := t.Execute(task.NewContext()); err != nil {
				if errorx.Cast(err) != nil {
					// FIXME: Map possible task errors and give suggestions.
					return err
				}
				return perrs.Trace(err)
			}

			if err := os.RemoveAll(meta.ClusterPath(clusterName)); err != nil {
				return perrs.Trace(err)
			}
			log.Infof("Destroyed DM cluster `%s` successfully", clusterName)
			return nil
		},
	}

	return cmd
}
*/
