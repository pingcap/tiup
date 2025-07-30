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
	"fmt"
	"path"
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/checkpoint"
	"github.com/pingcap/tiup/pkg/cluster/audit"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/tui"
	"github.com/spf13/cobra"
)

func newReplayCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "replay <audit-id>",
		Short: "Replay previous operation and skip successed steps",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			file := path.Join(spec.AuditDir(), args[0])
			if !checkpoint.HasCheckPoint() {
				if err := checkpoint.SetCheckPoint(file); err != nil {
					return errors.Annotate(err, "set checkpoint failed")
				}
			}

			args, err := audit.CommandArgs(file)
			if err != nil {
				return errors.Annotate(err, "read audit log failed")
			}

			if !skipConfirm {
				if err := tui.PromptForConfirmOrAbortError(
					"%s", fmt.Sprintf("Will replay the command `tiup dm %s`\nDo you want to continue? [y/N]: ", strings.Join(args[1:], " ")),
				); err != nil {
					return err
				}
			}

			rootCmd.SetArgs(args[1:])
			return rootCmd.Execute()
		},
	}

	return cmd
}
