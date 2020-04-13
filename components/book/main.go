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

	port, err := utils.GetFreePort("", 39989)
	if err != nil {
		return errors.New("cannot find free port")
	}

	http.Handle("/", http.FileServer(http.Dir(filepath.Join(home, "_book"))))
	if err := open.Run(fmt.Sprintf("http://localhost:%d", port)); err != nil {
		return err
	}
	return http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}
