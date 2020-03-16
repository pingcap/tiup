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
	"path"

	"github.com/pingcap-incubator/tiup/pkg/localdata"
)

func main() {
	installDir := os.Getenv(localdata.EnvNameComponentInstallDir)
	if installDir == "" {
		fmt.Printf("Envariable %s not set, plase make sure you run TiOPS with TiUP\n", localdata.EnvNameComponentInstallDir)
		os.Exit(-1)
	}
	if workDir := os.Getenv(localdata.EnvNameWorkDir); workDir != "" {
		os.Chdir(workDir)
	}
	cmd := exec.Command(path.Join(installDir, "tiops.py"), os.Args[1:]...)
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("PYTHONPATH=%s", path.Join(installDir, "deps")),
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Faild to run tiops.py: %s\n", err.Error())
		os.Exit(-1)
	}
}
