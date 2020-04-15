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
		os.Exit(1)
	}
}

func execute() error {
	lang := "en"

	rootCmd := &cobra.Command{
		Use:          "tiup doc",
		Short:        "The document abount TiDB",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			home := os.Getenv(localdata.EnvNameComponentInstallDir)
			if home == "" {
				return errors.New("component `doc` cannot run in standalone mode")
			}

			port, err := utils.GetFreePort("", 39989)
			if err != nil {
				return errors.New("cannot find free port")
			}

			http.Handle("/", http.FileServer(http.Dir(filepath.Join(home, "dist"))))
			if lang == "en" {
				if err := open.Run(fmt.Sprintf("http://localhost:%d/docs/stable/", port)); err != nil {
					return err
				}
			} else if lang == "cn" {
				if err := open.Run(fmt.Sprintf("http://localhost:%d/docs-cn/stable/", port)); err != nil {
					return err
				}
			} else {
				return errors.New("lang should be en or cn")
			}
			return http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
		},
	}

	rootCmd.Flags().StringVar(&lang, "lang", lang, "The language of the document: en/cn")
	return rootCmd.Execute()
}
