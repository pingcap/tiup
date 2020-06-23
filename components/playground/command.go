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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/components/playground/instance"
	"github.com/spf13/cobra"
)

// CommandType send to playground.
type CommandType string

// types of CommandType
const (
	ScaleInCommandType  CommandType = "scale-in"
	ScaleOutCommandType CommandType = "scale-out"
	DisplayCommandType  CommandType = "display"
)

// Command send to Playground.
type Command struct {
	CommandType CommandType
	PID         int // Set when scale-in
	ComponentID string
	instance.Config
}

func buildCommands(tp CommandType, opt *bootOptions) (cmds []Command) {
	for i := 0; i < opt.pd.Num; i++ {
		c := Command{
			CommandType: tp,
			ComponentID: "pd",
			Config:      opt.pd,
		}

		cmds = append(cmds, c)
	}
	for i := 0; i < opt.tikv.Num; i++ {
		c := Command{
			CommandType: tp,
			ComponentID: "tikv",
			Config:      opt.tikv,
		}

		cmds = append(cmds, c)
	}
	for i := 0; i < opt.pump.Num; i++ {
		c := Command{
			CommandType: tp,
			ComponentID: "pump",
			Config:      opt.pump,
		}

		cmds = append(cmds, c)
	}
	for i := 0; i < opt.tiflash.Num; i++ {
		c := Command{
			CommandType: tp,
			ComponentID: "tiflash",
			Config:      opt.tiflash,
		}

		cmds = append(cmds, c)
	}
	for i := 0; i < opt.tidb.Num; i++ {
		c := Command{
			CommandType: tp,
			ComponentID: "tidb",
			Config:      opt.tidb,
		}

		cmds = append(cmds, c)
	}
	for i := 0; i < opt.drainer.Num; i++ {
		c := Command{
			CommandType: tp,
			ComponentID: "drainer",
			Config:      opt.drainer,
		}

		cmds = append(cmds, c)
	}
	return
}

func newScaleOut() *cobra.Command {
	var opt bootOptions
	cmd := &cobra.Command{
		Use:     "scale-out instances",
		Example: "tiup playground scale-out --db 1",
		RunE: func(cmd *cobra.Command, args []string) error {
			num, err := scaleOut(args, &opt)
			if err != nil {
				return err
			}

			if num == 0 {
				return cmd.Help()
			}

			return nil
		},
		Hidden: false,
	}

	cmd.Flags().IntVarP(&opt.tidb.Num, "db", "", opt.tidb.Num, "TiDB instance number")
	cmd.Flags().IntVarP(&opt.tikv.Num, "kv", "", opt.tikv.Num, "TiKV instance number")
	cmd.Flags().IntVarP(&opt.pd.Num, "pd", "", opt.pd.Num, "PD instance number")
	cmd.Flags().IntVarP(&opt.tiflash.Num, "tiflash", "", opt.tiflash.Num, "TiFlash instance number")
	cmd.Flags().IntVarP(&opt.pump.Num, "pump", "", opt.pump.Num, "Pump instance number")
	cmd.Flags().IntVarP(&opt.drainer.Num, "drainer", "", opt.pump.Num, "Drainer instance number")

	cmd.Flags().StringVarP(&opt.tidb.Host, "db.host", "", opt.tidb.Host, "Playground TiDB host. If not provided, TiDB will still use `host` flag as its host")
	cmd.Flags().StringVarP(&opt.pd.Host, "pd.host", "", opt.pd.Host, "Playground PD host. If not provided, PD will still use `host` flag as its host")

	cmd.Flags().StringVarP(&opt.tidb.ConfigPath, "db.config", "", opt.tidb.ConfigPath, "TiDB instance configuration file")
	cmd.Flags().StringVarP(&opt.tikv.ConfigPath, "kv.config", "", opt.tikv.ConfigPath, "TiKV instance configuration file")
	cmd.Flags().StringVarP(&opt.pd.ConfigPath, "pd.config", "", opt.pd.ConfigPath, "PD instance configuration file")
	cmd.Flags().StringVarP(&opt.tidb.ConfigPath, "tiflash.config", "", opt.tidb.ConfigPath, "TiFlash instance configuration file")
	cmd.Flags().StringVarP(&opt.pump.ConfigPath, "pump.config", "", opt.pump.ConfigPath, "Pump instance configuration file")
	cmd.Flags().StringVarP(&opt.drainer.ConfigPath, "drainer.config", "", opt.drainer.ConfigPath, "Drainer instance configuration file")

	cmd.Flags().StringVarP(&opt.tidb.BinPath, "db.binpath", "", opt.tidb.BinPath, "TiDB instance binary path")
	cmd.Flags().StringVarP(&opt.tikv.BinPath, "kv.binpath", "", opt.tikv.BinPath, "TiKV instance binary path")
	cmd.Flags().StringVarP(&opt.pd.BinPath, "pd.binpath", "", opt.pd.BinPath, "PD instance binary path")
	cmd.Flags().StringVarP(&opt.tiflash.BinPath, "tiflash.binpath", "", opt.tiflash.BinPath, "TiFlash instance binary path")
	cmd.Flags().StringVarP(&opt.pump.BinPath, "pump.binpath", "", opt.pump.BinPath, "Pump instance binary path")
	cmd.Flags().StringVarP(&opt.drainer.BinPath, "drainer.binpath", "", opt.drainer.BinPath, "Drainer instance binary path")

	return cmd
}

func newScaleIn() *cobra.Command {
	var pids []int

	cmd := &cobra.Command{
		Use:     "scale-in a instance with specified pid",
		Example: "tiup playground scale-in --pid 234 # You can get pid by `tiup playground display`",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(pids) == 0 {
				return cmd.Help()
			}

			return scaleIn(pids)
		},
		Hidden: false,
	}

	cmd.Flags().IntSliceVar(&pids, "pid", nil, "pid of instance to be scale in")

	return cmd
}

func newDisplay() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "display the instances.",
		Hidden: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			return display(args)
		},
	}
	return cmd
}

func scaleIn(pids []int) error {
	port, err := targetTag()
	if err != nil {
		return errors.AddStack(err)
	}

	var cmds []Command
	for _, pid := range pids {
		c := Command{
			CommandType: ScaleInCommandType,
			PID:         pid,
		}
		cmds = append(cmds, c)
	}

	addr := "127.0.0.1:" + strconv.Itoa(port)
	return sendCommandsAndPrintResult(cmds, addr)
}

func scaleOut(args []string, opt *bootOptions) (num int, err error) {
	port, err := targetTag()
	if err != nil {
		return 0, errors.AddStack(err)
	}

	cmds := buildCommands(ScaleOutCommandType, opt)
	if len(cmds) == 0 {
		return 0, nil
	}

	addr := "127.0.0.1:" + strconv.Itoa(port)
	return len(cmds), sendCommandsAndPrintResult(cmds, addr)
}

func display(args []string) error {
	port, err := targetTag()
	if err != nil {
		return errors.AddStack(err)
	}
	c := Command{
		CommandType: DisplayCommandType,
	}

	addr := "127.0.0.1:" + strconv.Itoa(port)
	return sendCommandsAndPrintResult([]Command{c}, addr)
}

func sendCommandsAndPrintResult(cmds []Command, addr string) error {
	for _, cmd := range cmds {
		rc, err := requestCommand(cmd, addr)
		if err != nil {
			return errors.AddStack(err)
		}

		_, err = io.Copy(os.Stdout, rc)
		rc.Close()
		if err != nil {
			return errors.AddStack(err)
		}
	}

	return nil
}

func requestCommand(cmd Command, addr string) (r io.ReadCloser, err error) {
	data, err := json.Marshal(&cmd)
	if err != nil {
		return nil, errors.AddStack(err)
	}

	url := fmt.Sprintf("http://%s/command", addr)

	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, errors.AddStack(err)
	}

	return resp.Body, nil
}
