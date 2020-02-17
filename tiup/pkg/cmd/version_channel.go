package cmd

import (
	"fmt"
	"os"

	"github.com/AstroProfundis/tiup-demo/tiup/pkg/meta"
	"github.com/AstroProfundis/tiup-demo/tiup/pkg/utils"
	"github.com/spf13/cobra"
)

type channelCmd struct {
	*baseCmd
}

func newChannelCmd() *channelCmd {
	cmdVerChan := &channelCmd{
		newBaseCmd(&cobra.Command{
			Use:   "channel",
			Short: "Manage the update channel of TiDB components",
			Long: `Manage the update channel of TiDB components.
Channel specifies the update channel of components, default is 'stable':
  stable: the GA versions, ready for production
  beta: the pre-release testing versions, supposed to work but may have unfixed bugs
  nightly: the daily build of latest code, considered as unstable or even not work at all`,
			Run: func(cmd *cobra.Command, args []string) {
				cmd.Help()
			},
		}),
	}

	cmdVerChan.cmd.AddCommand(newChanList())
	cmdVerChan.cmd.AddCommand(newChanUse())
	return cmdVerChan
}

// newChanList
func newChanList() *cobra.Command {
	chanListCmd := &cobra.Command{
		Use:   "list",
		Short: "List available update channels",
		RunE: func(cmd *cobra.Command, args []string) error {
			compMeta, err := meta.ReadComponentList()
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Println("no available component list, try `tiup component list --refresh` to get latest online list.")
					return nil
				}
				return err
			}
			fmt.Println("Available update channels:")
			var chanTable [][]string
			chanTable = append(chanTable, []string{"Channel", "Latest Version", "Current"})

			currChan, err := meta.ReadVersionFile()
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			if len(compMeta.Stable) > 0 {
				line := []string{"stable", compMeta.Stable}
				if currChan.Chan == "stable" {
					line = append(line, currChan.Ver)
				}
				chanTable = append(chanTable, line)
			}
			if len(compMeta.Beta) > 0 {
				line := []string{"beta", compMeta.Beta}
				if currChan.Chan == "beta" {
					line = append(line, "*")
				}
				chanTable = append(chanTable, line)
			}
			//if len(compMeta.Nightly) > 0 {
			//	chanTable = append(chanTable, []string{"nightly", compMeta.Nightly})
			//}
			utils.PrintTable(chanTable, true)
			return nil
		},
	}

	return chanListCmd
}

// newChanUse
func newChanUse() *cobra.Command {
	channel := "stable"

	chanUseCmd := &cobra.Command{
		Use:   "use channel",
		Short: "Set the default update channel to use",
		Args: func(cmd *cobra.Command, args []string) error {
			argsLen := len(args)
			if argsLen < 1 {
				cmd.Help()
				return nil
			}

			if !meta.IsValidChannel(args[0]) {
				return fmt.Errorf("unknown channel %s", args[0])
			}
			channel = args[0]
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			compMeta, err := meta.ReadComponentList()
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Println("no available component list, try `tiup component list --refresh` to get latest online list.")
					return nil
				}
				return err
			}

			newVer := &meta.CurrVer{
				Chan: channel,
			}
			switch channel {
			case "stable":
				newVer.Ver = compMeta.Stable
			case "beta":
				newVer.Ver = compMeta.Beta
				//case "nightly":
				//	newVer.Ver = compMeta.Nightly
			}

			if err := meta.SaveCurrentVersion(newVer); err != nil {
				return nil
			}
			fmt.Printf("Default channel set to %s, current version is %s\n",
				newVer.Chan,
				newVer.Ver)
			return nil
		},
	}

	return chanUseCmd
}
