// Copyright 2022 PingCAP, Inc.
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
	"time"

	"github.com/spf13/cobra"
)

func newMetaCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "meta",
		Short: "backup/restore meta information",
	}

	var filePath string

	var metaBackupCmd = &cobra.Command{
		Use:   "backup <cluster-name>",
		Short: "backup topology and other information of a cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("please input cluster name")
			}
			if filePath == "" {
				filePath = "tiup-cluster_" + args[0] + "_metabackup_" + time.Now().Format(time.RFC3339) + ".tar.gz"
			}
			err := cm.BackupClusterMeta(args[0], filePath)
			if err == nil {
				log.Infof("successfully backup meta of cluster %s on %s", args[0], filePath)
			}
			return err
		},
	}
	metaBackupCmd.Flags().StringVar(&filePath, "file", "", "filepath of output tarball")

	var metaRestoreCmd = &cobra.Command{
		Use:   "restore <cluster-name> <backup-file>",
		Short: "restore topology and other information of a cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return fmt.Errorf("please input cluster name and path to the backup file")
			}
			return cm.RestoreClusterMeta(args[0], args[1], skipConfirm)
		},
	}

	cmd.AddCommand(metaBackupCmd)
	cmd.AddCommand(metaRestoreCmd)

	return cmd
}
