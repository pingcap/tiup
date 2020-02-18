package cmd

import (
	"fmt"
	"os"

	"github.com/AstroProfundis/tiup-demo/tiup/pkg/meta"
	"github.com/AstroProfundis/tiup-demo/tiup/pkg/utils"
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
			RunE: func(cmd *cobra.Command, args []string) error {
				return showCurrentVersion()
			},
		}),
	}

	cmdVersion.cmd.AddCommand(newChannelCmd().cmd)
	return cmdVersion
}

func showCurrentVersion() error {
	currChan, err := meta.ReadVersionFile()
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("default version not set, try `tiup version channel use` to set a update channel.")
			return nil
		}
		return err
	}
	curOS, curArch := utils.GetPlatform()
	fmt.Printf("Current Version: %s %s-%s (%s)\n",
		currChan.Ver,
		curOS,
		curArch,
		currChan.Chan)
	return nil
}
