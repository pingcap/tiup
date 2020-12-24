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
	"path"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cliutil"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	"github.com/pingcap/tiup/pkg/cluster/manager"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

func newDeployCmd() *cobra.Command {
	opt := manager.DeployOptions{
		IdentityFile: path.Join(utils.UserHome(), ".ssh", "id_rsa"),
	}
	cmd := &cobra.Command{
		Use:          "deploy <cluster-name> <version> <topology.yaml>",
		Short:        "Deploy a DM cluster for production",
		Long:         "Deploy a DM cluster for production. SSH connection will be used to deploy files, as well as creating system users for running the service.",
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
			version, err := utils.FmtVer(args[1])
			if err != nil {
				return err
			}
			topoFile := args[2]

			if err := supportVersion(version); err != nil {
				return err
			}

			return cm.Deploy(clusterName, version, topoFile, opt, nil, skipConfirm, gOpt)
		},
	}

	cmd.Flags().StringVarP(&opt.User, "user", "u", utils.CurrentUser(), "The user name to login via SSH. The user must has root (or sudo) privilege.")
	cmd.Flags().StringVarP(&opt.IdentityFile, "identity_file", "i", opt.IdentityFile, "The path of the SSH identity file. If specified, public key authentication will be used.")
	cmd.Flags().BoolVarP(&opt.UsePassword, "password", "p", false, "Use password of target hosts. If specified, password authentication will be used.")

	return cmd
}

func supportVersion(vs string) error {
	if !semver.IsValid(vs) {
		return nil
	}

	majorMinor := semver.MajorMinor(vs)
	if semver.Compare(majorMinor, "v2.0") < 0 {
		return errors.Errorf("Only support version not less than v2.0")
	}

	return nil
}
