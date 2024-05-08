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

func buildCommands(tp CommandType, opt *BootOptions) (cmds []Command) {
	commands := []struct {
		comp string
		instance.Config
	}{
		{"pd", opt.PD},
		{"tso", opt.TSO},
		{"scheduling", opt.Scheduling},
		{"tikv", opt.TiKV},
		{"pump", opt.Pump},
		{"tiflash", opt.TiFlash},
		{"tiproxy", opt.TiProxy},
		{"tidb", opt.TiDB},
		{"ticdc", opt.TiCDC},
		{"tikv-cdc", opt.TiKVCDC},
		{"drainer", opt.Drainer},
	}

	for _, cmd := range commands {
		for i := 0; i < cmd.Num; i++ {
			c := Command{
				CommandType: tp,
				ComponentID: cmd.comp,
				Config:      cmd.Config,
			}

			cmds = append(cmds, c)
		}
	}
	return
}

func newScaleOut() *cobra.Command {
	var opt BootOptions
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

	cmd.Flags().IntVarP(&opt.TiDB.Num, "db", "", opt.TiDB.Num, "TiDB instance number")
	cmd.Flags().IntVarP(&opt.TiKV.Num, "kv", "", opt.TiKV.Num, "TiKV instance number")
	cmd.Flags().IntVarP(&opt.PD.Num, "pd", "", opt.PD.Num, "PD instance number")
	cmd.Flags().IntVarP(&opt.TSO.Num, "tso", "", opt.TSO.Num, "TSO instance number")
	cmd.Flags().IntVarP(&opt.Scheduling.Num, "scheduling", "", opt.Scheduling.Num, "Scheduling instance number")
	cmd.Flags().IntVarP(&opt.TiFlash.Num, "tiflash", "", opt.TiFlash.Num, "TiFlash instance number")
	cmd.Flags().IntVarP(&opt.TiProxy.Num, "tiproxy", "", opt.TiProxy.Num, "TiProxy instance number")
	cmd.Flags().IntVarP(&opt.TiCDC.Num, "ticdc", "", opt.TiCDC.Num, "TiCDC instance number")
	cmd.Flags().IntVarP(&opt.TiKVCDC.Num, "kvcdc", "", opt.TiKVCDC.Num, "TiKV-CDC instance number")
	cmd.Flags().IntVarP(&opt.Pump.Num, "pump", "", opt.Pump.Num, "Pump instance number")
	cmd.Flags().IntVarP(&opt.Drainer.Num, "drainer", "", opt.Pump.Num, "Drainer instance number")
	cmd.Flags().StringVarP(&opt.TiDB.Host, "db.host", "", opt.TiDB.Host, "Playground TiDB host. If not provided, TiDB will still use `host` flag as its host")
	cmd.Flags().StringVarP(&opt.PD.Host, "pd.host", "", opt.PD.Host, "Playground PD host. If not provided, PD will still use `host` flag as its host")
	cmd.Flags().StringVarP(&opt.TSO.Host, "tso.host", "", opt.TSO.Host, "Playground TSO host. If not provided, TSO will still use `host` flag as its host")
	cmd.Flags().StringVarP(&opt.Scheduling.Host, "scheduling.host", "", opt.Scheduling.Host, "Playground Scheduling host. If not provided, Scheduling will still use `host` flag as its host")
	cmd.Flags().StringVarP(&opt.TiProxy.Host, "tiproxy.host", "", opt.PD.Host, "Playground TiProxy host. If not provided, TiProxy will still use `host` flag as its host")

	cmd.Flags().StringVarP(&opt.TiDB.ConfigPath, "db.config", "", opt.TiDB.ConfigPath, "TiDB instance configuration file")
	cmd.Flags().StringVarP(&opt.TiKV.ConfigPath, "kv.config", "", opt.TiKV.ConfigPath, "TiKV instance configuration file")
	cmd.Flags().StringVarP(&opt.PD.ConfigPath, "pd.config", "", opt.PD.ConfigPath, "PD instance configuration file")
	cmd.Flags().StringVarP(&opt.TSO.ConfigPath, "tso.config", "", opt.TSO.ConfigPath, "TSO instance configuration file")
	cmd.Flags().StringVarP(&opt.Scheduling.ConfigPath, "scheduling.config", "", opt.Scheduling.ConfigPath, "Scheduling instance configuration file")
	cmd.Flags().StringVarP(&opt.TiFlash.ConfigPath, "tiflash.config", "", opt.TiFlash.ConfigPath, "TiFlash instance configuration file")
	cmd.Flags().StringVarP(&opt.TiProxy.ConfigPath, "tiproxy.config", "", opt.TiProxy.ConfigPath, "TiProxy instance configuration file")
	cmd.Flags().StringVarP(&opt.Pump.ConfigPath, "pump.config", "", opt.Pump.ConfigPath, "Pump instance configuration file")
	cmd.Flags().StringVarP(&opt.Drainer.ConfigPath, "drainer.config", "", opt.Drainer.ConfigPath, "Drainer instance configuration file")

	cmd.Flags().StringVarP(&opt.TiDB.BinPath, "db.binpath", "", opt.TiDB.BinPath, "TiDB instance binary path")
	cmd.Flags().StringVarP(&opt.TiKV.BinPath, "kv.binpath", "", opt.TiKV.BinPath, "TiKV instance binary path")
	cmd.Flags().StringVarP(&opt.PD.BinPath, "pd.binpath", "", opt.PD.BinPath, "PD instance binary path")
	cmd.Flags().StringVarP(&opt.TSO.BinPath, "tso.binpath", "", opt.TSO.BinPath, "TSO instance binary path")
	cmd.Flags().StringVarP(&opt.Scheduling.BinPath, "scheduling.binpath", "", opt.Scheduling.BinPath, "Scheduling instance binary path")
	cmd.Flags().StringVarP(&opt.TiFlash.BinPath, "tiflash.binpath", "", opt.TiFlash.BinPath, "TiFlash instance binary path")
	cmd.Flags().StringVarP(&opt.TiProxy.BinPath, "tiproxy.binpath", "", opt.TiProxy.BinPath, "TiProxy instance binary path")
	cmd.Flags().StringVarP(&opt.TiCDC.BinPath, "ticdc.binpath", "", opt.TiCDC.BinPath, "TiCDC instance binary path")
	cmd.Flags().StringVarP(&opt.TiKVCDC.BinPath, "kvcdc.binpath", "", opt.TiKVCDC.BinPath, "TiKVCDC instance binary path")
	cmd.Flags().StringVarP(&opt.Pump.BinPath, "pump.binpath", "", opt.Pump.BinPath, "Pump instance binary path")
	cmd.Flags().StringVarP(&opt.Drainer.BinPath, "drainer.binpath", "", opt.Drainer.BinPath, "Drainer instance binary path")

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
		return err
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

func scaleOut(args []string, opt *BootOptions) (num int, err error) {
	port, err := targetTag()
	if err != nil {
		return 0, err
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
		return err
	}
	c := Command{
		CommandType: DisplayCommandType,
	}

	addr := "127.0.0.1:" + strconv.Itoa(port)
	return sendCommandsAndPrintResult([]Command{c}, addr)
}

func sendCommandsAndPrintResult(cmds []Command, addr string) error {
	for _, cmd := range cmds {
		data, err := json.Marshal(&cmd)
		if err != nil {
			return errors.AddStack(err)
		}

		url := fmt.Sprintf("http://%s/command", addr)

		resp, err := http.Post(url, "application/json", bytes.NewReader(data))
		if err != nil {
			return errors.AddStack(err)
		}
		defer resp.Body.Close()

		_, err = io.Copy(os.Stdout, resp.Body)
		if err != nil {
			return errors.AddStack(err)
		}
	}

	return nil
}
