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

package cmd

import (
	"fmt"
	"io/ioutil"

	//"github.com/pingcap-incubator/tiops/pkg/task"
	"github.com/pingcap-incubator/tiops/pkg/meta"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

func newDisplayCmd() *cobra.Command {
	var (
		//clusterName  string
		topologyFile string
	)

	cmd := &cobra.Command{
		Use:    "display",
		Short:  "Display information of a TiDB cluster",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			var topo meta.TopologySpecification

			yamlFile, err := ioutil.ReadFile(topologyFile)
			if err != nil {
				return err
			}

			if err = yaml.Unmarshal(yamlFile, &topo); err != nil {
				return err
			}

			topoData, err := yaml.Marshal(topo)
			fmt.Printf("%s", topoData)
			return nil
		},
	}

	//cmd.Flags().StringVar(&clusterName, "name", "", "name of TiDB cluster")
	cmd.Flags().StringVar(&topologyFile, "topology", "", "path to the topology file")

	return cmd
}
