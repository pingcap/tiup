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
	"database/sql"
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/crypto/rand"
	"github.com/pingcap/tiup/pkg/proxy"
	"github.com/spf13/cobra"

	// for sql/driver
	_ "github.com/go-sql-driver/mysql"
)

func newStartCmd() *cobra.Command {
	var initPasswd bool

	cmd := &cobra.Command{
		Use:   "start <cluster-name>",
		Short: "Start a TiDB cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			if err := validRoles(gOpt.Roles); err != nil {
				return err
			}

			clusterName := args[0]
			clusterReport.ID = scrubClusterName(clusterName)
			teleCommand = append(teleCommand, scrubClusterName(clusterName))

			if err := cm.StartCluster(clusterName, gOpt, func(b *task.Builder, metadata spec.Metadata) {
				b.UpdateTopology(
					clusterName,
					tidbSpec.Path(clusterName),
					metadata.(*spec.ClusterMeta),
					nil, /* deleteNodeIds */
				)
			}); err != nil {
				return err
			}

			// init password
			if initPasswd {
				pwd, err := initPassword(clusterName)
				if err != nil {
					log.Errorf("Failed to set root password of TiDB database to '%s'", pwd)
					if strings.Contains(strings.ToLower(err.Error()), "error 1045") {
						log.Errorf("Initializing is only working when the root password is empty")
						log.Errorf(color.YellowString("Did you already set root password before?"))
					}
					return err
				}
				log.Warnf("The root password of TiDB database has been changed.")
				fmt.Printf("The new password is: '%s'.\n", color.HiYellowString(pwd)) // use fmt to avoid printing to audit log
				log.Warnf("Copy and record it to somewhere safe, %s, and will not be stored.", color.HiRedString("it is only displayed once"))
				log.Warnf("The generated password %s.", color.HiRedString("can NOT be get and shown again"))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&initPasswd, "init", false, "Initialize a secure root password for the database")
	cmd.Flags().StringSliceVarP(&gOpt.Roles, "role", "R", nil, "Only start specified roles")
	cmd.Flags().StringSliceVarP(&gOpt.Nodes, "node", "N", nil, "Only start specified nodes")

	return cmd
}

func initPassword(clusterName string) (string, error) {
	metadata, err := spec.ClusterMetadata(clusterName)
	if err != nil {
		return "", err
	}
	tcpProxy := proxy.GetTCPProxy()

	// generate password
	pwd, err := rand.Password(18)
	if err != nil {
		return pwd, err
	}

	var lastErr error
	for _, spec := range metadata.Topology.TiDBServers {
		spec := spec
		endpoint := fmt.Sprintf("%s:%d", spec.Host, spec.Port)
		if tcpProxy != nil {
			closeC := tcpProxy.Run([]string{endpoint})
			defer tcpProxy.Close(closeC)
			endpoint = tcpProxy.GetEndpoints()[0]
		}
		db, err := createDB(endpoint)
		if err != nil {
			lastErr = err
			continue
		}
		defer db.Close()

		sqlStr := fmt.Sprintf("SET PASSWORD FOR 'root'@'%%' = '%s'; FLUSH PRIVILEGES;", pwd)
		_, err = db.Exec(sqlStr)
		if err != nil {
			lastErr = err
			continue
		}
		return pwd, nil
	}

	return pwd, lastErr
}

func createDB(endpoint string) (db *sql.DB, err error) {
	dsn := fmt.Sprintf("root:@tcp(%s)/?charset=utf8mb4,utf8&multiStatements=true", endpoint)
	db, err = sql.Open("mysql", dsn)

	return
}
