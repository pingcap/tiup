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
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/pingcap/tiup/pkg/cluster/ansible"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/tui"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/spf13/cobra"
)

func newImportCmd() *cobra.Command {
	var (
		ansibleDir        string
		inventoryFileName string
		ansibleCfgFile    string
		rename            string
		noBackup          bool
	)

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import an exist TiDB cluster from TiDB-Ansible",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Use current directory as ansibleDir by default
			if ansibleDir == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				ansibleDir = cwd
			}

			ctx := ctxt.New(
				context.Background(),
				gOpt.Concurrency,
				log,
			)

			// migrate cluster metadata from Ansible inventory
			clsName, clsMeta, inv, err := ansible.ReadInventory(ctx, ansibleDir, inventoryFileName)
			if err != nil {
				return err
			}

			// Rename the imported cluster
			if rename != "" {
				clsName = rename
			}
			if clsName == "" {
				return fmt.Errorf("cluster name should not be empty")
			}

			exist, err := tidbSpec.Exist(clsName)
			if err != nil {
				return err
			}

			if exist {
				return errDeployNameDuplicate.
					New("Cluster name '%s' is duplicated", clsName).
					WithProperty(tui.SuggestionFromFormat(
						fmt.Sprintf("Please use --rename `NAME` to specify another name (You can use `%s list` to see all clusters)", tui.OsArgs0())))
			}

			// prompt for backups
			backupDir := spec.ClusterPath(clsName, "ansible-backup")
			backupFile := filepath.Join(ansibleDir, fmt.Sprintf("tiup-%s.bak", inventoryFileName))
			prompt := fmt.Sprintf("The ansible directory will be moved to %s after import.", backupDir)
			if noBackup {
				log.Infof("The '--no-backup' flag is set, the ansible directory will be kept at its current location.")
				prompt = fmt.Sprintf("The inventory file will be renamed to %s after import.", backupFile)
			}
			log.Warnf("TiDB-Ansible and TiUP Cluster can NOT be used together, please DO NOT try to use ansible to manage the imported cluster anymore to avoid metadata conflict.")
			log.Infof(prompt)
			if !skipConfirm {
				err = tui.PromptForConfirmOrAbortError("Do you want to continue? [y/N]: ")
				if err != nil {
					return err
				}
			}

			if !skipConfirm {
				err = tui.PromptForConfirmOrAbortError(
					"Prepared to import TiDB %s cluster %s.\nDo you want to continue? [y/N]:",
					clsMeta.Version,
					clsName)
				if err != nil {
					return err
				}
			}

			// parse config and import nodes
			if err = ansible.ParseAndImportInventory(
				ctx,
				ansibleDir,
				ansibleCfgFile,
				clsMeta,
				inv,
				gOpt.SSHTimeout,
				gOpt.SSHType,
			); err != nil {
				return err
			}

			// copy SSH key to TiUP profile directory
			if err = utils.MkdirAll(spec.ClusterPath(clsName, "ssh"), 0755); err != nil {
				return err
			}
			srcKeyPathPriv := ansible.SSHKeyPath()
			srcKeyPathPub := srcKeyPathPriv + ".pub"
			dstKeyPathPriv := spec.ClusterPath(clsName, "ssh", "id_rsa")
			dstKeyPathPub := dstKeyPathPriv + ".pub"
			if err = utils.Copy(srcKeyPathPriv, dstKeyPathPriv); err != nil {
				return err
			}
			if err = utils.Copy(srcKeyPathPub, dstKeyPathPub); err != nil {
				return err
			}

			// copy config files form deployment servers
			if err = ansible.ImportConfig(ctx, clsName, clsMeta, gOpt); err != nil {
				return err
			}

			// copy config detail to meta file
			if err = ansible.LoadConfig(clsName, clsMeta); err != nil {
				return err
			}

			if err = spec.SaveClusterMeta(clsName, clsMeta); err != nil {
				return err
			}

			// comment config to avoid duplicated copy
			if err = ansible.CommentConfig(clsName); err != nil {
				return err
			}

			// backup ansible files
			if noBackup {
				// rename original TiDB-Ansible inventory file
				if err = utils.Move(filepath.Join(ansibleDir, inventoryFileName), backupFile); err != nil {
					return err
				}
				log.Infof("Ansible inventory renamed to %s.", color.HiCyanString(backupFile))
			} else {
				// move original TiDB-Ansible directory to a staged location
				if err = utils.Move(ansibleDir, backupDir); err != nil {
					return err
				}
				log.Infof("Ansible inventory saved in %s.", color.HiCyanString(backupDir))
			}

			log.Infof("Cluster %s imported.", clsName)
			fmt.Printf("Try `%s` to show node list and status of the cluster.\n",
				color.HiYellowString("%s display %s", tui.OsArgs0(), clsName))
			return nil
		},
	}

	cmd.Flags().StringVarP(&ansibleDir, "dir", "d", "", "The path to TiDB-Ansible directory")
	cmd.Flags().StringVar(&inventoryFileName, "inventory", ansible.AnsibleInventoryFile, "The name of inventory file")
	cmd.Flags().StringVar(&ansibleCfgFile, "ansible-config", ansible.AnsibleConfigFile, "The path to ansible.cfg")
	cmd.Flags().StringVarP(&rename, "rename", "r", "", "Rename the imported cluster to `NAME`")
	cmd.Flags().BoolVar(&noBackup, "no-backup", false, "Don't backup ansible dir, useful when there're multiple inventory files")

	return cmd
}
