package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

type installCmd struct {
	*baseCmd
}

func newInstCmd() *installCmd {
	var (
		version       string
		componentList []string
	)

	cmdInst := &installCmd{
		newBaseCmd(&cobra.Command{
			Use:   "install",
			Short: "Install TiDB component(s) of specific version",
			Long:  `Install some or all components of TiDB of specific version.`,
			RunE: func(cmd *cobra.Command, args []string) error {
				return installComponent(version, componentList)
			},
		}),
	}

	cmdInst.cmd.Flags().StringVarP(&version, "version", "v", "", "Specify the version of component(s) to install.")
	cmdInst.cmd.Flags().StringSliceVarP(&componentList, "component", "c", []string{}, "List of component(s) to install.")

	return cmdInst
}

func installComponent(ver string, list []string) error {
	fmt.Printf("%s, %v\n", ver, list)
	return nil
}
