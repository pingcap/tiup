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
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/pingcap-incubator/tiup/pkg/localdata"
	"github.com/pingcap-incubator/tiup/pkg/utils"
	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/cobra"
)

func main() {
	if err := execute(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}

func execute() error {
	home := os.Getenv(localdata.EnvNameComponentInstallDir)
	if home == "" {
		return errors.New("component `book` cannot run in standalone mode")
	}

	var lang string
	rootCmd := &cobra.Command{
		Use: "tiup mirrors <target-dir>",
		Example: `  tiup book --lang=en       # Open the English version of e-book <TiDB in Action>
  tiup book --lang=zh_CN    # Open the Chinese version of e-book <TiDB in Action>`,
		Short:        "Build a local mirrors and download all selected components",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if lang == "" {
				return cmd.Help()
			}

			if lang != "zh_CN" {
				return fmt.Errorf("language `%s` unspported (supported language: zh_CN)", lang)
			}

			port, err := utils.GetFreePort("", 39989)
			if err != nil {
				return errors.New("cannot find free port")
			}

			http.Handle("/", http.FileServer(http.Dir(filepath.Join(home, "_book"))))
			if err := open.Run(fmt.Sprintf("http://localhost:%d", port)); err != nil {
				return err
			}
			return http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
		},
	}

	rootCmd.Flags().StringVarP(&lang, "lang", "L", "", "Specify the `language` of e-book")

	return rootCmd.Execute()
}
