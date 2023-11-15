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
	"os/exec"
	"runtime"

	"github.com/pingcap/tiup/pkg/utils"
	"github.com/spf13/cobra"
)

func main() {
	if err := execute(); err != nil {
		fmt.Println("Packaging component failed:", err)
		os.Exit(1)
	}
}

type packageOptions struct {
	goos       string
	goarch     string
	dir        string
	name       string
	version    string
	entry      string
	desc       string
	standalone bool
	hide       bool
}

func execute() error {
	options := packageOptions{}

	rootCmd := &cobra.Command{
		Use:          "tiup package target",
		Short:        "Package a tiup component and generate package directory",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}

			return pack(args, options)
		},
	}

	// some arguments are not used anymore, we keep them to make it compatible
	// with legacy CI jobs
	rootCmd.Flags().StringVar(&options.goos, "os", runtime.GOOS, "Target OS of the package")
	rootCmd.Flags().StringVar(&options.goarch, "arch", runtime.GOARCH, "Target ARCH of the package")
	rootCmd.Flags().StringVarP(&options.dir, "", "C", "", "Change directory before compress")
	rootCmd.Flags().StringVar(&options.name, "name", "", "Name of the package (required)")
	rootCmd.Flags().StringVar(&options.version, "release", "", "Version of the package (required)")
	rootCmd.Flags().StringVar(&options.entry, "entry", "", "(deprecated) Entry point of the package")
	rootCmd.Flags().StringVar(&options.desc, "desc", "", "(deprecated) Description of the package")
	rootCmd.Flags().BoolVar(&options.standalone, "standalone", false, "(deprecated) Can the component run standalone")
	rootCmd.Flags().BoolVar(&options.hide, "hide", false, "(deprecated) Don't show the component in `tiup list`")

	_ = rootCmd.MarkFlagRequired("name")
	_ = rootCmd.MarkFlagRequired("release")

	return rootCmd.Execute()
}

func pack(targets []string, options packageOptions) error {
	if err := utils.MkdirAll("package", 0755); err != nil {
		return err
	}

	// tar -czf package/{name}-{version}-{goos}-{goarch}.tar.gz target
	return packTarget(targets, options)
}

func packTarget(targets []string, options packageOptions) error {
	file := fmt.Sprintf("package/%s-%s-%s-%s.tar.gz", options.name, options.version, options.goos, options.goarch)
	args := []string{"-czf", file}
	if options.dir != "" {
		args = append(args, "-C", options.dir)
	}
	cmd := exec.Command("tar", append(args, targets...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Println(cmd.Args)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("package target: %s", err.Error())
	}
	return nil
}
