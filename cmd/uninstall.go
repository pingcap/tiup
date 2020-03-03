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
	"path/filepath"
	"strings"

	"github.com/c4pt0r/tiup/pkg/localdata"
	"github.com/spf13/cobra"
)

func newUninstallCmd() *cobra.Command {
	var all bool
	cmdUnInst := &cobra.Command{
		Use:   "uninstall <component1>:<version>",
		Short: "Uninstall components or versions of a component",
		Long: `If you specify a version number, uninstall the specified version of
the component. You must use --all explicitly if you want to remove all
components or versions which are installed. You can uninstall multiple
component or multiple version of a component at once`,
		RunE: func(cmd *cobra.Command, args []string) error {
			switch {
			case len(args) > 0:
				return removeComponents(args, all)
			case len(args) == 0 && all:
				return os.RemoveAll(profile.Path(localdata.ComponentParentDir))
			default:
				return cmd.Help()
			}
		},
	}
	cmdUnInst.Flags().BoolVar(&all, "all", false, "Remove all components or versions.")
	return cmdUnInst
}

func removeComponents(specs []string, all bool) error {
	for _, spec := range specs {
		var path string
		if strings.Contains(spec, ":") {
			parts := strings.SplitN(spec, ":", 2)
			path = profile.Path(filepath.Join(localdata.ComponentParentDir, parts[0], parts[1]))
		} else {
			if !all {
				fmt.Printf("Use `tiup remove %s --all` if you want to remove all versions.\n", spec)
				continue
			}
			path = profile.Path(filepath.Join(localdata.ComponentParentDir, spec))
		}
		err := os.RemoveAll(path)
		if err != nil {
			return err
		}
	}
	return nil
}
