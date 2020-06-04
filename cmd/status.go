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
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/tui"
	"github.com/pingcap/tiup/pkg/utils"
	gops "github.com/shirou/gopsutil/process"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "List the status of instantiated components",
		RunE: func(cmd *cobra.Command, args []string) error {
			env := environment.GlobalEnv()
			if len(args) > 0 {
				return cmd.Help()
			}
			return showStatus(env)
		},
	}
	return cmd
}

func showStatus(env *environment.Environment) error {
	var table [][]string
	table = append(table, []string{"Name", "Component", "PID", "Status", "Created Time", "Directory", "Binary", "Args"})
	if dataDir := env.LocalPath(localdata.DataParentDir); utils.IsExist(dataDir) {
		dirs, err := ioutil.ReadDir(dataDir)
		if err != nil {
			return err
		}
		for _, dir := range dirs {
			if !dir.IsDir() {
				continue
			}

			process, err := env.Profile().ReadMetaFile(dir.Name())
			if err != nil {
				return err
			}
			if process == nil {
				// If the path doesn't contain the meta file, which means startup interrupted
				_ = os.RemoveAll(env.LocalPath(filepath.Join(localdata.DataParentDir, dir.Name())))
				continue
			}

			status := "TERM"
			if exist, err := gops.PidExists(int32(process.Pid)); err == nil && exist {
				status = "RUNNING"
			}
			table = append(table, []string{
				dir.Name(),
				process.Component,
				strconv.Itoa(process.Pid),
				status,
				process.CreatedTime,
				process.Dir,
				process.Exec,
				strings.Join(process.Args, " "),
			})
		}
	}
	tui.PrintTable(table, true)
	return nil
}
