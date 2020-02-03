package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

type unInstCmd struct {
	*baseCmd
}

func newUnInstCmd() *unInstCmd {
	var (
		version       string
		componentList []string
	)

	cmdUnInst := &unInstCmd{
		newBaseCmd(&cobra.Command{
			Use:   "uninstall",
			Short: "Uninstall TiDB component(s) of specific version",
			Long:  `Uninstall some or all components of TiDB of specific version.`,
			RunE: func(cmd *cobra.Command, args []string) error {
				return unInstallComponent(version, componentList)
			},
		}),
	}

	cmdUnInst.cmd.Flags().StringVarP(&version, "version", "v", "", "Specify the version of component(s) to uninstall.")
	cmdUnInst.cmd.Flags().StringSliceVarP(&componentList, "component", "c", []string{}, "List of component(s) to uninstall.")

	return cmdUnInst
}

func unInstallComponent(ver string, list []string) error {
	fmt.Printf("%s, %v\n", ver, list)
	return nil
}
