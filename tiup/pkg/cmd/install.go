package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/AstroProfundis/tiup-demo/tiup/pkg/utils"
	"github.com/spf13/cobra"
)

var (
	componentListURL      = "https://repo.hoshi.at/tmp/components.json"
	installedListFilename = "installed.json"
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

type installedComp struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
	//Path string `json:"path,omitempty"`
}

func installComponent(ver string, list []string) error {
	meta, err := readComponentList()
	if err != nil {
		return err
	}

	var installCnt int
	for _, comp := range list {
		installed, err := checkInstalledComponent(comp, ver)
		if err != nil {
			return err
		}
		if installed {
			fmt.Printf("%s %s already installed, skip.\n", comp, ver)
			return nil
		}

		url, checksum := getComponentURL(meta.Components, ver, comp)
		if len(url) > 0 {
			if err := utils.DownloadFile(url, checksum); err != nil {
				return err
			}
			if err := saveInstalledList(&installedComp{
				Name:    comp,
				Version: ver,
			}); err != nil {
				return err
			}
			fmt.Printf("Installed %s %s.\n", comp, ver)
			installCnt++
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

func getInstalledList() ([]installedComp, error) {
	var list []installedComp
	var err error

	data, err := utils.ReadFile(installedListFilename)
	if err != nil {
		if os.IsNotExist(err) {
			return list, nil
		}
		return nil, err
	}
	if err = json.Unmarshal(data, &list); err != nil {
		return nil, err
	}

	return list, err
}

func saveInstalledList(comp *installedComp) error {
	currList, err := getInstalledList()
	if err != nil {
		return err
	}

	for _, instComp := range currList {
		if instComp.Name == comp.Name &&
			instComp.Version == comp.Version {
			return fmt.Errorf("%s %s is already installed",
				instComp.Name, instComp.Version)
		}
	}
	newList := append(currList, *comp)
	return utils.WriteJSON(installedListFilename, newList)
}

func checkInstalledComponent(name string, ver string) (bool, error) {
	currList, err := getInstalledList()
	if err != nil {
		return false, err
	}

	for _, instComp := range currList {
		if instComp.Name == name &&
			instComp.Version == ver {
			return true, nil
		}
	}
	return false, nil
}
