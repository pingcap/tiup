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
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pingcap-incubator/tiup/pkg/localdata"
	"github.com/pingcap-incubator/tiup/pkg/meta"
	"github.com/pingcap-incubator/tiup/pkg/set"
	"github.com/pingcap-incubator/tiup/pkg/utils"
	gops "github.com/shirou/gopsutil/process"
	"github.com/spf13/cobra"
)

func newCleanCmd(env *meta.Environment) *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "clean <name>",
		Short: "Clean the data of instantiated components",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !all {
				return cmd.Help()
			}
			return cleanData(env, args, all)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Clean all data of instantiated components")
	return cmd
}

func cleanData(env *meta.Environment, names []string, all bool) error {
	dataDir := env.LocalPath(localdata.DataParentDir)
	if utils.IsNotExist(dataDir) {
		return nil
	}
	dirs, err := ioutil.ReadDir(dataDir)
	if err != nil {
		return err
	}
	clean := set.NewStringSet(names...)
	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}
		if !all && !clean.Exist(dir.Name()) {
			continue
		}

		process, err := env.Profile().ReadMetaFile(dir.Name())
		if err != nil {
			return err
		}
		if process == nil {
			continue
		}

		if p, err := gops.NewProcess(int32(process.Pid)); err == nil {
			fmt.Printf("Kill instance of `%s`, pid: %v\n", process.Component, process.Pid)
			if err := p.Kill(); err != nil {
				return err
			}
		}

		if err := os.RemoveAll(filepath.Join(dataDir, dir.Name())); err != nil {
			return err
		}

		fmt.Printf("Clean instance of `%s`, directory: %s\n", process.Component, process.Dir)
	}
	return nil
}
