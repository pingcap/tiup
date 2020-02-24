package cmd

import (
	"fmt"

	"github.com/c4pt0r/tiup/pkg/version"
	"github.com/spf13/cobra"
)

type baseCmd struct {
	cmd *cobra.Command
}

func newBaseCmd(cmd *cobra.Command) *baseCmd {
	return &baseCmd{cmd: cmd}
}

func (c *baseCmd) getCmd() *cobra.Command {
	return c.cmd
}

func (c *baseCmd) Execute() error {
	return c.cmd.Execute()
}

var rootCmd *baseCmd

func init() {
	var printVersion bool

	rootCmd = newBaseCmd(&cobra.Command{
		Use:   "tiup",
		Short: "Download and install TiDB components from command line",
		Long: `The tiup utility is a command line tool that can help downloading
and installing TiDB components to the local system.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if printVersion {
				fmt.Println(version.NewTiUPVersion())
				fmt.Println(version.NewTiUPBuildInfo())
			} else {
				cmd.Help()
			}
			return nil
		},
	})

	rootCmd.cmd.Flags().BoolVarP(&printVersion, "version", "V", false, "Show tiup version and quit")

	rootCmd.cmd.AddCommand(
		newComponentCmd().getCmd(),
		newVersionCmd().getCmd(),
		newLaunchCmd().getCmd(),
		newProcCmd().getCmd(),
		newShowCmd().getCmd(),
	)
}

// Execute parses the command line argumnts and calls proper functions
func Execute() error {
	return rootCmd.Execute()
}
