package cmd

import (
	"fmt"

	"github.com/AstroProfundis/tiup-demo/tiup/pkg/utils"
	"github.com/spf13/cobra"
)

var (
	componentListURL = "https://repo.hoshi.at/tmp/components.json"
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
	meta, err := readComponentList()
	if err != nil {
		return err
	}

	var installCnt int
	for _, comp := range list {
		url, checksum := getComponentURL(meta.Components, ver, comp)
		if len(url) > 0 {
			err := utils.DownloadFile(url, checksum)
			if err != nil {
				return err
			}
			// TODO: save installed list to file
			fmt.Printf("Installed %s %s.\n", comp, ver)
			installCnt += 1
		}
	}
	fmt.Printf("Installed %d component(s).\n", installCnt)
	return nil
}

func getComponentURL(list []compItem, ver string, comp string) (string, string) {
	for _, compMetaItem := range list {
		if comp != compMetaItem.Name {
			continue
		}
		for _, item := range compMetaItem.VersionList {
			if ver == item.Version {
				return item.URL, item.SHA256
			}
		}
	}
	return "", ""
}
