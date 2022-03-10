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
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/audit"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/spf13/cobra"
)

//  retainDay number of days to keep audit logs for deletion
var retainDays int

func newAuditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit [audit-id]",
		Short: "Show audit log of cluster operation",
		RunE: func(cmd *cobra.Command, args []string) error {
			switch len(args) {
			case 0:
				return audit.ShowAuditList(spec.AuditDir())
			case 1:
				return audit.ShowAuditLog(spec.AuditDir(), args[0])
			default:
				return cmd.Help()
			}
		},
	}
	cmd.AddCommand(newAuditCleanupCmd())
	return cmd
}

func newAuditCleanupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "cleanup cluster audit logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			if retainDays < 0 {
				return errors.Errorf("retain-days cannot be less than 0")
			}

			err := audit.DeleteAuditLog(spec.AuditDir(), retainDays, skipConfirm, gOpt.DisplayMode)
			if err != nil {
				return err
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&retainDays, "retain-days", 60, "Number of days to keep audit logs for deletion")
	return cmd
}
