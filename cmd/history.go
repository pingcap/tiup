// Copyright 2022 PingCAP, Inc.
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
	"strconv"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/tui"
	"github.com/spf13/cobra"
)

// newHistoryCmd  history
func newHistoryCmd() *cobra.Command {
	rows := 100
	var displayMode string
	var all bool
	cmd := &cobra.Command{
		Use:   "history <rows>",
		Short: "Display the historical execution record of TiUP, displays 100 lines by default",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				r, err := strconv.Atoi(args[0])
				if err == nil {
					rows = r
				} else {
					return fmt.Errorf("%s: numeric argument required", args[0])
				}
			}

			env := environment.GlobalEnv()
			rows, err := env.GetHistory(rows, all)
			if err != nil {
				return err
			}

			if displayMode == "json" {
				for _, r := range rows {
					rBytes, err := json.Marshal(r)
					if err != nil {
						continue
					}
					fmt.Println(string(rBytes))
				}
				return nil
			}
			var table [][]string
			table = append(table, []string{"Date", "Command", "Code"})

			for _, r := range rows {
				table = append(table, []string{
					r.Date.Format("2006-01-02T15:04:05"),
					r.Command,
					strconv.Itoa(r.Code),
				})
			}
			tui.PrintTable(table, true)
			fmt.Printf("history log save path: %s\n", env.LocalPath(environment.HistoryDir))
			return nil
		},
	}
	cmd.Flags().StringVar(&displayMode, "format", "default", "The format of output, available values are [default, json]")
	cmd.Flags().BoolVar(&all, "all", false, "Display all execution history")
	cmd.AddCommand(newHistoryCleanupCmd())
	return cmd
}

func newHistoryCleanupCmd() *cobra.Command {
	var retainDays int
	var all bool
	var skipConfirm bool
	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "delete all execution history",
		RunE: func(cmd *cobra.Command, args []string) error {
			if retainDays < 0 {
				return errors.Errorf("retain-days cannot be less than 0")
			}

			if all {
				retainDays = 0
			}

			env := environment.GlobalEnv()
			return env.DeleteHistory(retainDays, skipConfirm)
		},
	}

	cmd.Flags().IntVar(&retainDays, "retain-days", 60, "Number of days to keep history for deletion")
	cmd.Flags().BoolVar(&all, "all", false, "Delete all history")
	cmd.Flags().BoolVarP(&skipConfirm, "yes", "y", false, "Skip all confirmations and assumes 'yes'")
	return cmd
}
