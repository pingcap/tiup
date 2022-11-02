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
	"errors"
	"fmt"

	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/proxy"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	// for sql/driver
	_ "github.com/go-sql-driver/mysql"
)

func newTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "_test <cluster-name>",
		Short:  "test toolkit",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}

			clusterName := args[0]

			exist, err := tidbSpec.Exist(clusterName)
			if err != nil {
				return err
			}

			if !exist {
				return perrs.Errorf("cannot start non-exists cluster %s", clusterName)
			}

			metadata, err := spec.ClusterMetadata(clusterName)
			if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) {
				return err
			}

			tcpProxy := proxy.GetTCPProxy()
			switch args[1] {
			case "writable":
				return writable(metadata.Topology, tcpProxy)
			case "data":
				return data(metadata.Topology, tcpProxy)
			default:
				fmt.Println("unknown command: ", args[1])
				return cmd.Help()
			}
		},
	}

	return cmd
}

// To check if test.ti_cluster has data
func data(topo *spec.Specification, tcpProxy *proxy.TCPProxy) error {
	errg, _ := errgroup.WithContext(context.Background())

	for _, spec := range topo.TiDBServers {
		spec := spec
		endpoint := utils.JoinHostPort(spec.Host, spec.Port)
		errg.Go(func() error {
			if tcpProxy != nil {
				closeC := tcpProxy.Run([]string{endpoint})
				defer tcpProxy.Close(closeC)
				endpoint = tcpProxy.GetEndpoints()[0]
			}
			db, err := createDB(endpoint)
			if err != nil {
				return err
			}

			row := db.QueryRow("select count(*) from test.ti_cluster")
			count := 0
			if err := row.Scan(&count); err != nil {
				return err
			}

			if count == 0 {
				return errors.New("table test.ti_cluster is empty")
			}

			fmt.Printf("check data %s success\n", utils.JoinHostPort(spec.Host, spec.Port))
			return nil
		})
	}

	return errg.Wait()
}

func writable(topo *spec.Specification, tcpProxy *proxy.TCPProxy) error {
	errg, _ := errgroup.WithContext(context.Background())

	for _, spec := range topo.TiDBServers {
		spec := spec
		endpoint := utils.JoinHostPort(spec.Host, spec.Port)
		errg.Go(func() error {
			if tcpProxy != nil {
				closeC := tcpProxy.Run([]string{endpoint})
				defer tcpProxy.Close(closeC)
				endpoint = tcpProxy.GetEndpoints()[0]
			}
			db, err := createDB(endpoint)
			if err != nil {
				return err
			}

			_, err = db.Exec("create table if not exists test.ti_cluster(id int AUTO_INCREMENT primary key, v int)")
			if err != nil {
				return err
			}

			_, err = db.Exec("insert into test.ti_cluster (v) values(1)")
			if err != nil {
				return err
			}

			fmt.Printf("write %s success\n", utils.JoinHostPort(spec.Host, spec.Port))
			return nil
		})
	}

	return errg.Wait()
}
