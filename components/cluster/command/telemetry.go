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
	"fmt"

	"github.com/pingcap/tiup/pkg/telemetry"
	"github.com/spf13/cobra"
)

func newTelemetryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "_telemetry",
		Short:  "telemetry",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}

			switch args[0] {
			case "node_info":
				return nodeInfo(context.Background())
			default:
				fmt.Println("unknown command: ", args[1])
				return cmd.Help()
			}
		},
	}

	return cmd
}

// nodeInfo display telemetry.NodeInfo in stdout.
func nodeInfo(ctx context.Context) error {
	info := new(telemetry.NodeInfo)
	err := telemetry.FillNodeInfo(ctx, info)
	if err != nil {
		return err
	}

	text, err := telemetry.NodeInfoToText(info)
	if err != nil {
		return err
	}

	fmt.Println(text)
	return nil
}
