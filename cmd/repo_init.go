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
	//"fmt"
	"os"

	"github.com/pingcap-incubator/tiup/pkg/meta"
	"github.com/pingcap-incubator/tiup/pkg/utils"

	//"github.com/pingcap-incubator/tiup/pkg/repository"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func newRepoInitCmd(env *meta.Environment) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initialise an empty repository",
		Long: `Initialise an empty TiUP repository at given path. If path is not specified, the
current working directory (".") will be used.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				repoPath string
				err      error
			)
			if len(args) == 0 {
				if repoPath, err = os.Getwd(); err != nil {
					return err
				}
			} else {
				repoPath = args[0]
			}

			// create the target path if not exist
			if utils.IsNotExist(repoPath) {
				if err = os.Mkdir(repoPath, 0755); err != nil {
					return err
				}
			}
			// init requires an empty path to use
			empty, err := utils.IsEmptyDir(repoPath)
			if err != nil {
				return err
			}
			if !empty {
				return errors.Errorf("the target path '%s' is not an empty directory", repoPath)
			}

			return initRepo(repoPath)
		},
	}

	return cmd
}

func initRepo(path string) error {
	return nil
}
