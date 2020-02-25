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
	"encoding/json"
	"fmt"
	"github.com/c4pt0r/tiup/pkg/utils"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	"strings"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "List status of all running components",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
}

const (
	processListFilename = "processes.json"
)

func newProcCmd() *cobra.Command {
	cmdProc := &cobra.Command{
		Use:   "process",
		Short: "Manage processes of components",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	cmdProc.AddCommand(newProcListCmd())
	return cmdProc
}

type compProcess struct {
	Pid  int      `json:"pid,omitempty"`  // PID of the process
	Exec string   `json:"exec,omitempty"` // Path to the binary
	Args []string `json:"args,omitempty"` // Command line arguments
	Dir  string   `json:"dir,omitempty"`  // Working directory
}

type compProcessList []compProcess

// Launch executes the process
func (p *compProcess) Launch() error {
	var err error

	dir := utils.MustDir(p.Dir)
	p.Pid, err = utils.Exec(nil, nil, dir, p.Exec, p.Args...)
	if err != nil {
		return err
	}
	return nil
}

func getProcessList() (compProcessList, error) {
	var list compProcessList
	var err error

	data, err := utils.ReadFile(processListFilename)
	if err != nil {
		if os.IsNotExist(err) {
			return list, nil
		}
		return nil, err
	}
	if err = json.Unmarshal(data, &list); err != nil {
		return nil, err
	}

	return list, err
}

func saveProcessList(pl *compProcessList) error {
	return utils.WriteJSON(processListFilename, pl)
}

func saveProcessToList(p *compProcess) error {
	currList, err := getProcessList()
	if err != nil {
		return err
	}

	for _, currProc := range currList {
		if currProc.Pid == p.Pid {
			return fmt.Errorf("process %d already exist", p.Pid)
		}
	}

	newList := append(currList, *p)
	return saveProcessList(&newList)
}

func newProcListCmd() *cobra.Command {
	cmdProcList := &cobra.Command{
		Use:   "list",
		Short: "Show process list",
		Long: `Show current process list, note that this is the list saved when
the process launched, the actual process might already exited and no longer running.`,
		RunE: showProcessList,
	}
	return cmdProcList
}

func showProcessList(cmd *cobra.Command, args []string) error {
	procList, err := getProcessList()
	if err != nil {
		return err
	}

	fmt.Println("Launched processes:")
	var procTable [][]string
	procTable = append(procTable, []string{"Process", "PID", "Working Dir", "Argument"})
	for _, proc := range procList {
		procTable = append(procTable, []string{
			filepath.Base(proc.Exec),
			fmt.Sprint(proc.Pid),
			proc.Dir,
			strings.Join(proc.Args, " "),
		})
	}

	utils.PrintTable(procTable, true)
	return nil
}
