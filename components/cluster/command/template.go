// Copyright 2021 PingCAP, Inc.
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

package command

import (
	"fmt"
	"path"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/embed"
	"github.com/spf13/cobra"
)

// TemplateOptions contains the options for print topology template.
type TemplateOptions struct {
	Full    bool // print full template
	MultiDC bool // print template for deploying to multiple data center
}

func newTemplateCmd() *cobra.Command {
	opt := TemplateOptions{}

	cmd := &cobra.Command{
		Use:   "template",
		Short: "Print topology template",
		RunE: func(cmd *cobra.Command, args []string) error {
			if opt.Full && opt.MultiDC {
				return errors.New("at most one of 'full' and 'multi-dc' can be specified")
			}
			name := "minimal.yaml"
			if opt.Full {
				name = "topology.example.yaml"
			} else if opt.MultiDC {
				name = "multi-dc.yaml"
			}

			fp := path.Join("templates", "examples", name)
			tpl, err := embed.ReadFile(fp)
			if err != nil {
				return err
			}

			fmt.Println(string(tpl))
			return nil
		},
	}

	cmd.Flags().BoolVar(&opt.Full, "full", false, "Print the full topology template for TiDB cluster.")
	cmd.Flags().BoolVar(&opt.MultiDC, "multi-dc", false, "Print template for deploying to multiple data center.")

	return cmd
}
