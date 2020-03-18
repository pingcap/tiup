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
	"github.com/pingcap-incubator/tiops/pkg/task"
	"github.com/spf13/cobra"
)

func newDeploy() *cobra.Command {
	// for test
	var (
		user string
		host string
		key  string
	)
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy a cluster for production",
		RunE: func(cmd *cobra.Command, args []string) error {
			t := task.NewBuilder().
				RootSSH(host, key, user).
				EnvInit(host).
				Build()
			return t.Execute(&task.Context{})
		},
	}

	cmd.Flags().StringVar(&key, "key", ".ssh/id_rsa", "keypath")
	cmd.Flags().StringVar(&host, "host", "", "deploy to host")
	cmd.Flags().StringVar(&user, "user", "root", "system user root")
	return cmd
}
