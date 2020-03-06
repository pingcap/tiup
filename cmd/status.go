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

	"github.com/c4pt0r/tiup/pkg/localdata"
	"github.com/c4pt0r/tiup/pkg/tui"
	"github.com/c4pt0r/tiup/pkg/utils"
	gops "github.com/shirou/gopsutil/process"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "List the status of running components",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return cmd.Help()
			}
			return showStatus()
		},
	}
	return cmd
}

func showStatus() error {
	var table [][]string
	table = append(table, []string{"Name", "Component", "PID", "Status", "Created Time", "Directory", "Binary"})
	if dataDir := profile.Path(localdata.DataParentDir); utils.IsExist(dataDir) {
		dirs, err := ioutil.ReadDir(dataDir)
		if err != nil {
			return err
		}
		for _, dir := range dirs {
			if !dir.IsDir() {
				continue
			}
			metaFile := filepath.Join(localdata.DataParentDir, dir.Name(), localdata.MetaFilename)

			// If the path doesn't contain the meta file, which means startup interrupted
			if utils.IsNotExist(profile.Path(metaFile)) {
				_ = os.RemoveAll(profile.Path(filepath.Join(localdata.DataParentDir, dir.Name())))
				continue
			}

			var process process
			err := profile.ReadJSON(metaFile, &process)
			if err != nil {
				return err
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
			})
		}
	}
	tui.PrintTable(table, true)
	return nil
}
