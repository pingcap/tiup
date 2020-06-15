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
	"io/ioutil"
	"os"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cliutil"
	"github.com/pingcap/tiup/pkg/cluster/meta"
	tiuputils "github.com/pingcap/tiup/pkg/utils"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all clusters",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listCluster()
		},
	}
	return cmd
}

func listCluster() error {
	clusterDir := meta.ProfilePath(meta.TiOpsClusterDir)
	clusterTable := [][]string{
		// Header
		{"Name", "User", "Version", "Path", "PrivateKey"},
	}
	fileInfos, err := ioutil.ReadDir(clusterDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, fi := range fileInfos {
		if tiuputils.IsNotExist(meta.ClusterPath(fi.Name(), meta.MetaFileName)) {
			continue
		}
		metadata, err := meta.DMMetadata(fi.Name())
		if err != nil {
			return errors.Trace(err)
		}

		clusterTable = append(clusterTable, []string{
			fi.Name(),
			metadata.User,
			metadata.Version,
			meta.ClusterPath(fi.Name()),
			meta.ClusterPath(fi.Name(), "ssh", "id_rsa"),
		})
	}

	cliutil.PrintTable(clusterTable, true)
	return nil
}
*/
