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
			Short: "Manage versions of TiDB components",
			Long: `Manage the global version of TiDB components, if unset, the
latest version of 'stable' channel is used.`,
			Run: func(cmd *cobra.Command, args []string) {
				cmd.Help()
			},
		}),
	}

	cmdVersion.cmd.AddCommand(newChannelCmd().cmd)
	return cmdVersion
}
