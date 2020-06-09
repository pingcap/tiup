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
	"bytes"
	"io"
	"io/ioutil"
	"os"

	"github.com/fatih/color"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cliutil"
	"github.com/pingcap/tiup/pkg/cluster/edit"
	"github.com/pingcap/tiup/pkg/cluster/meta"
	"github.com/pingcap/tiup/pkg/logger"
	"github.com/pingcap/tiup/pkg/logger/log"
	tiuputils "github.com/pingcap/tiup/pkg/utils"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

func newEditConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit-config <cluster-name>",
		Short: "Edit TiDB cluster config",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			clusterName := args[0]
			teleCommand = append(teleCommand, scrubClusterName(clusterName))
			if tiuputils.IsNotExist(meta.ClusterPath(clusterName, meta.MetaFileName)) {
				return errors.Errorf("cannot start non-exists cluster %s", clusterName)
			}

			logger.EnableAuditLog()
			metadata, err := meta.ClusterMetadata(clusterName)
			if err != nil {
				return err
			}

			return editTopo(clusterName, metadata)
		},
	}

	return cmd
}

// 1. Write Topology to a temporary file.
// 2. Open file in editor.
// 3. Check and update Topology.
// 4. Save meta file.
func editTopo(clusterName string, metadata *meta.ClusterMeta) error {
	data, err := yaml.Marshal(metadata.Topology)
	if err != nil {
		return errors.AddStack(err)
	}

	file, err := ioutil.TempFile(os.TempDir(), "*")
	if err != nil {
		return errors.AddStack(err)
	}

	name := file.Name()

	_, err = io.Copy(file, bytes.NewReader(data))
	if err != nil {
		return errors.AddStack(err)
	}

	err = file.Close()
	if err != nil {
		return errors.AddStack(err)
	}

	err = edit.OpenFileInEditor(name)
	if err != nil {
		return errors.AddStack(err)
	}

	// Now user finish editing the file.
	newData, err := ioutil.ReadFile(name)
	if err != nil {
		return errors.AddStack(err)
	}

	newTopo := new(meta.TopologySpecification)
	err = yaml.UnmarshalStrict(newData, newTopo)
	if err != nil {
		log.Infof("Failed to parse topology file: %v", err)
		return errors.AddStack(err)
	}

	if bytes.Equal(data, newData) {
		log.Infof("The file has nothing changed")
		return nil
	}

	edit.ShowDiff(string(data), string(newData), os.Stdout)

	if !skipConfirm {
		if err := cliutil.PromptForConfirmOrAbortError(
			color.HiYellowString("Please check change highlight above, do you want to apply the change? [y/N]:"),
		); err != nil {
			return err
		}
	}

	log.Infof("Apply the change...")

	metadata.Topology = newTopo
	err = meta.SaveClusterMeta(clusterName, metadata)
	if err != nil {
		return errors.Annotate(err, "failed to save")
	}

	log.Infof("Apply change successfully, please use `%s reload %s [-N <nodes>] [-R <roles>]` to reload config.", cliutil.OsArgs0(), clusterName)

	return nil
}
