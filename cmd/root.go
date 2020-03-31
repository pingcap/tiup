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
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/pingcap-incubator/tiops/pkg/base52"
	"github.com/pingcap-incubator/tiops/pkg/flags"
	"github.com/pingcap-incubator/tiops/pkg/log"
	"github.com/pingcap-incubator/tiops/pkg/meta"
	"github.com/pingcap-incubator/tiops/pkg/utils"
	"github.com/pingcap-incubator/tiops/pkg/version"
	"github.com/pingcap-incubator/tiup/pkg/localdata"
	tiupmeta "github.com/pingcap-incubator/tiup/pkg/meta"
	"github.com/pingcap-incubator/tiup/pkg/repository"
	"github.com/spf13/cobra"
)

var auditConfig struct {
	buffer *bytes.Buffer
	enable bool
}

var rootCmd *cobra.Command

func init() {
	// Initialize the audit configuration
	auditConfig.buffer = bytes.NewBufferString(strings.Join(os.Args, " ") + "\n")

	// Initialize the global variables
	flags.ShowBacktrace = len(os.Getenv("TIUP_BACKTRACE")) > 0
	cobra.EnableCommandSorting = false

	rootCmd = &cobra.Command{
		Use:           filepath.Base(os.Args[0]),
		Short:         "Deploy a TiDB cluster for production",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version.NewTiOpsVersion().FullInfo(),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := meta.Initialize(); err != nil {
				return err
			}
			log.SetOutput(io.MultiWriter(os.Stdout, auditConfig.buffer))
			return tiupmeta.InitRepository(repository.Options{
				GOOS:   "linux",
				GOARCH: "amd64",
			})
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(color.Reset)
			return tiupmeta.Repository().Mirror().Close()
		},
	}

	rootCmd.AddCommand(
		newDeploy(),
		newStartCmd(),
		newStopCmd(),
		newRestartCmd(),
		newScaleInCmd(),
		newScaleOutCmd(),
		newDestroyCmd(),
		newUpgradeCmd(),
		newExecCmd(),
		newDisplayCmd(),
		newListCmd(),
		newAuditCmd(),
		newImportCmd(),
	)
}

// Execute executes the root command
func Execute() {
	// Switch current work directory if running in TiUP component mode
	if wd := os.Getenv(localdata.EnvNameWorkDir); wd != "" {
		if err := os.Chdir(wd); err != nil {
			log.Warnf("Switch work directory to %s failed: %v", wd, err)
		}
	}

	var code int
	err := rootCmd.Execute()
	if err != nil {
		if flags.ShowBacktrace {
			log.Output(color.RedString("Error: %+v", err))
		} else {
			log.Output(color.RedString("Error: %v", err))
		}
		code = 1
	}
	if auditConfig.enable {
		auditDir := meta.ProfilePath(meta.TiOpsAuditDir)
		if err := utils.CreateDir(auditDir); err != nil {
			fmt.Println(color.RedString("Create audit directory error: %v", err))
		} else {
			auditFilePath := meta.ProfilePath(meta.TiOpsAuditDir, base52.Encode(time.Now().Unix()))
			_ = ioutil.WriteFile(auditFilePath, auditConfig.buffer.Bytes(), os.ModePerm)
		}
	}
	os.Exit(code)
}
