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

	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/tui"
	"github.com/spf13/cobra"
)

// newHistoryCmd  history
func newHistoryCmd() *cobra.Command {
	var rows int
	var displayMode string
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Display the historical execution record of TiUP",
		RunE: func(cmd *cobra.Command, args []string) error {
			env := environment.GlobalEnv()
			rows, err := env.GetHistory(rows)
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
			return nil
		},
	}
	cmd.Flags().StringVar(&displayMode, "format", "default", "(EXPERIMENTAL) The format of output, available values are [default, json]")
	cmd.Flags().IntVarP(&rows, "rows", "r", 60, "If the specified version was already installed, force a reinstallation")

	cmd.AddCommand(newHistoryCleanupCmd())
	return cmd
}

func newHistoryCleanupCmd() *cobra.Command {
	// var retainDays int
	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "cleanup cluster audit logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			// if retainDays < 0 {
			// 	return errors.Errorf("retain-days cannot be less than 0")
			// }

			env := environment.GlobalEnv()
			return env.DeleteHistory()
		},
	}

	// cmd.Flags().IntVar(&retainDays, "retain-days", 60, "Number of days to keep audit logs for deletion")
	return cmd
}
