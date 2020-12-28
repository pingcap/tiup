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
	"os"

	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/spf13/cobra"
)

var envList = []string{
	localdata.EnvNameHome,
	localdata.EnvNameSSHPassPrompt,
	localdata.EnvNameSSHPath,
	localdata.EnvNameSCPPath,
	localdata.EnvNameKeepSourceTarget,
	localdata.EnvNameMirrorSyncScript,
	localdata.EnvNameLogPath,
	localdata.EnvNameDebug,
}

func newEnvCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env [name1...N]",
		Short: "Show the list of system environment variable that related to TiUP",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				showEnvList(true, envList...)
				return nil
			}
			showEnvList(false, args...)
			return nil
		},
	}

	return cmd
}

func showEnvList(withKey bool, names ...string) {
	for _, name := range names {
		if withKey {
			fmt.Printf("%s=\"%s\"\n", name, os.Getenv(name))
		} else {
			fmt.Printf("%s\n", os.Getenv(name))
		}
	}
}
