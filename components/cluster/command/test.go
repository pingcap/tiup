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
	"database/sql"
	"errors"
	"fmt"

	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/meta"
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
			if utils.IsNotExist(meta.ClusterPath(clusterName, meta.MetaFileName)) {
				return perrs.Errorf("cannot start non-exists cluster %s", clusterName)
			}

			metadata, err := meta.ClusterMetadata(clusterName)
			if err != nil && !errors.Is(perrs.Cause(err), meta.ValidateErr) {
				return err
			}

			switch args[1] {
			case "writable":
				return writable(metadata.Topology)
			default:
				fmt.Println("unknown command: ", args[1])
				return cmd.Help()
			}
		},
	}

	return cmd
}

func createDB(spec meta.TiDBSpec) (db *sql.DB, err error) {
	dsn := fmt.Sprintf("root:@tcp(%s:%d)/?charset=utf8mb4,utf8&multiStatements=true", spec.Host, spec.Port)
	db, err = sql.Open("mysql", dsn)

	return
}

func writable(topo *meta.ClusterSpecification) error {
	errg, _ := errgroup.WithContext(context.Background())

	for _, spec := range topo.TiDBServers {
		spec := spec
		errg.Go(func() error {
			db, err := createDB(spec)
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

			fmt.Printf("write %s:%d success\n", spec.Host, spec.Port)
			return nil
		})
	}

	return errg.Wait()
}
