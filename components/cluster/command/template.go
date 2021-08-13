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
	Local   bool // print local template
}

// This is used to identify how many bool type options are set, so that an
// error can be throw if more than one is given.
func sumBool(b ...bool) int {
	n := 0
	for _, v := range b {
		if v {
			n++
		}
	}
	return n
}

func newTemplateCmd() *cobra.Command {
	opt := TemplateOptions{}

	cmd := &cobra.Command{
		Use:   "template",
		Short: "Print topology template",
		RunE: func(cmd *cobra.Command, args []string) error {
			if sumBool(opt.Full, opt.MultiDC, opt.Local) > 1 {
				return errors.New("at most one of 'full', 'multi-dc', or 'local' can be specified")
			}
			name := "minimal.yaml"
			switch {
			case opt.Full:
				name = "topology.example.yaml"
			case opt.MultiDC:
				name = "multi-dc.yaml"
			case opt.Local:
				name = "local.yaml"
			}

			fp := path.Join("examples", "cluster", name)
			tpl, err := embed.ReadExample(fp)
			if err != nil {
				return err
			}

			fmt.Println(string(tpl))
			return nil
		},
	}

	cmd.Flags().BoolVar(&opt.Full, "full", false, "Print the full topology template for TiDB cluster.")
	cmd.Flags().BoolVar(&opt.MultiDC, "multi-dc", false, "Print template for deploying to multiple data center.")
	cmd.Flags().BoolVar(&opt.Local, "local", false, "Print template for deploying a simple cluster locally.")

	return cmd
}
