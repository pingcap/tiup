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

package main

import (
	"fmt"
	"os"

	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/spf13/cobra"
)

func main() {
	addr := "0.0.0.0:8989"
	upstream := "https://tiup-mirrors.pingcap.com"
	indexKey := ""
	snapshotKey := ""
	timestampKey := ""

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s <root-dir>", os.Args[0]),
		Short: "bootstrap a mirror server",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			s, err := newServer(args[0], upstream, indexKey, snapshotKey, timestampKey)
			if err != nil {
				return err
			}

			return s.run(addr)
		},
	}
	cmd.Flags().StringVarP(&addr, "addr", "", addr, "addr to listen")
	cmd.Flags().StringVarP(&indexKey, "index", "", "", "specific the private key for index")
	cmd.Flags().StringVarP(&snapshotKey, "snapshot", "", "", "specific the private key for snapshot")
	cmd.Flags().StringVarP(&timestampKey, "timestamp", "", "", "specific the private key for timestamp")
	cmd.Flags().StringVarP(&upstream, "upstream", "", upstream, "specific the upstream mirror")

	if err := cmd.Execute(); err != nil {
		log.Errorf("Execute command: %s", err.Error())
	}
}
