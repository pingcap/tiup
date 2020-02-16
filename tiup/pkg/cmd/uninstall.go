package cmd

import (
	"fmt"

	"github.com/AstroProfundis/tiup-demo/tiup/pkg/utils"
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
			Use:     "uninstall <component1> [component2...N] <version>",
			Short:   "Uninstall TiDB component(s) of specific version",
			Long:    `Uninstall some or all components of TiDB of specific version.`,
			Example: "tiup component uninstall tidb-core 3.0.8",
			Args: func(cmd *cobra.Command, args []string) error {
				argsLen := len(args)
				if argsLen < 2 {
					cmd.Help()
					return nil
				}
				var err error
				version, err = utils.FmtVer(args[argsLen-1])
				if err != nil {
					return err
				}
				componentList = args[:argsLen-1]
				return nil
			},
			RunE: func(cmd *cobra.Command, args []string) error {
				return uninstallComponent(version, componentList)
			},
		}),
	}

	return cmdUnInst
}

func uninstallComponent(ver string, list []string) error {
	for _, comp := range list {
		installed, err := checkInstalledComponent(comp, ver)
		if err != nil {
			return err
		}
		if !installed {
			fmt.Printf("%s %s is not installed, skip.\n", comp, ver)
			continue
		}
		// do actual removal here
		// removeFromSystem()
		if err = removeFromInstalledList(comp, ver); err != nil {
			return err
		}
		fmt.Printf("%s %v uninstalled.\n", comp, ver)
	}
	return nil
}

func removeFromInstalledList(name string, ver string) error {
	currList, err := getInstalledList()
	if err != nil {
		return err
	}

	var newList []installedComp
	for i, instComp := range currList {
		if instComp.Name == name &&
			instComp.Version == ver {
			newList = append(currList[:i], currList[i+1:]...)
			break
		}
	}
	return utils.WriteJSON(installedListFilename, newList)
}
