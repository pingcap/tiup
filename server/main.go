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

	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/version"
	"github.com/spf13/cobra"
)

func main() {
	addr := "0.0.0.0:8989"
	keyDir := ""
	upstream := "https://tiup-mirrors.pingcap.com"

	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s <root-dir>", os.Args[0]),
		Short:   "bootstrap a mirror server",
		Version: version.NewTiUPVersion().String(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			s, err := newServer(args[0], keyDir, upstream)
			if err != nil {
				return err
			}

			return s.run(addr)
		},
	}
	cmd.Flags().StringVarP(&addr, "addr", "", addr, "addr to listen")
	cmd.Flags().StringVarP(&keyDir, "key-dir", "", keyDir, "specify the directory where stores the private keys")
	cmd.Flags().StringVarP(&upstream, "upstream", "", upstream, "specify the upstream mirror")

	if err := cmd.Execute(); err != nil {
		logprinter.Errorf("Execute command: %s", err.Error())
	}
}
