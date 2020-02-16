package cmd

import (
	"github.com/spf13/cobra"
)

type versionCmd struct {
	*baseCmd
}

func newVersionCmd() *versionCmd {
	cmdVersion := &versionCmd{
		newBaseCmd(&cobra.Command{
			Use:   "version",
			Short: "Manage TiDB versions",
			Run: func(cmd *cobra.Command, args []string) {
				cmd.Help()
			},
		}),
	}

	//cmdVersion.cmd.AddCommand(newListComponentCmd().cmd)
	return cmdVersion
}
