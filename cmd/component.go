package cmd

import (
	"github.com/spf13/cobra"
)

type componentCmd struct {
	*baseCmd
}

func newComponentCmd() *componentCmd {
	cmdComponent := &componentCmd{
		newBaseCmd(&cobra.Command{
			Use:   "component",
			Short: "Manage TiDB related components",
			Run: func(cmd *cobra.Command, args []string) {
				cmd.Help()
			},
		}),
	}

	cmdComponent.cmd.AddCommand(newListComponentCmd().cmd)
	cmdComponent.cmd.AddCommand(newInstCmd().cmd)
	cmdComponent.cmd.AddCommand(newUnInstCmd().cmd)
	return cmdComponent
}
