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

func newTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "test",
		Short:  "Run shell command on host in the tidb cluster",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			t := task.NewBuilder().RootSSH("127.0.0.1", "/Users/huangjiahao/.ssh/id_rsa", "huangjiahao").
				CopyFile("/tmp/a", "127.0.0.1", "/tmp/b").
				Build()
			ctx := task.NewContext()
			err := t.Execute(ctx)

			return err
		},
	}
	return cmd
}
