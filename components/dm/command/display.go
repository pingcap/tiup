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
	"errors"
	"fmt"

	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/spf13/cobra"
)

func newDisplayCmd() *cobra.Command {
	var (
		clusterName     string
		showVersionOnly bool
		statusTimeout   uint64
	)
	cmd := &cobra.Command{
		Use:   "display <cluster-name>",
		Short: "Display information of a DM cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			gOpt.APITimeout = statusTimeout
			clusterName = args[0]

			if showVersionOnly {
				metadata, err := spec.ClusterMetadata(clusterName)
				if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) {
					return err
				}
				fmt.Println(metadata.Version)
				return nil
			}

			return cm.Display(clusterName, gOpt)
		},
	}

	cmd.Flags().StringSliceVarP(&gOpt.Roles, "role", "R", nil, "Only display specified roles")
	cmd.Flags().StringSliceVarP(&gOpt.Nodes, "node", "N", nil, "Only display specified nodes")
	cmd.Flags().BoolVar(&showVersionOnly, "version", false, "Only display DM cluster version")
	cmd.Flags().BoolVar(&gOpt.ShowUptime, "uptime", false, "Display DM with uptime")
	cmd.Flags().Uint64Var(&statusTimeout, "status-timeout", 10, "Timeout in seconds when getting node status")

	return cmd
}
