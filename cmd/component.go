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

	cmdComponent.cmd.AddCommand(newListComponentCmd().getCmd())
	cmdComponent.cmd.AddCommand(newInstCmd().getCmd())
	cmdComponent.cmd.AddCommand(newUnInstCmd().getCmd())
	return cmdComponent
}
