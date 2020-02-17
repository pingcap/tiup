package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path"

	"github.com/AstroProfundis/tiup-demo/tiup/pkg/utils"
	"github.com/spf13/cobra"
)

var (
	componentListURL      = "https://repo.hoshi.at/tmp/components.json"
	installedListFilename = "installed.json"

	specifiedHomeEnvKey = "TIUP_HOME"
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
			Use:     "install <component1> [component2...N] <version>",
			Short:   "Install TiDB component(s) of specific version",
			Long:    `Install some or all components of TiDB of specific version.`,
			Example: "tiup component install tidb-core v3.0.8",
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
				return installComponent(version, componentList)
			},
		}),
	}

	return cmdInst
}

type installedComp struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
	Path    string `json:"path,omitempty"`
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
			// make sure we have correct download path
			profileDir := os.Getenv(specifiedHomeEnvKey)
			if len(profileDir) == 0 {
				profileDir = utils.ProfileDir()
			}

			toDir := utils.MustDir(path.Join(profileDir, "download/"))
			tarball := ""
			if tarball, err = utils.DownloadFileWithProgress(url, toDir); err != nil {
				return err
			}

			// validate checksum of downloaded tarball
			fmt.Printf("Validating checksum of downloaded file...")
			valid, err := utils.ValidateSHA256(tarball, checksum)
			if err != nil {
				return err
			}
			if !valid {
				return fmt.Errorf("checksum validation failed for %s", tarball)
			}
			fmt.Printf("done.\n")

			toDir = utils.MustDir(path.Join(profileDir, "bin/"))
			fmt.Printf("Decompressing...")
			if err = utils.Untar(tarball, toDir); err != nil {
				return err
			}
			if err := saveInstalledList(&installedComp{
				Name:    comp,
				Version: ver,
				Path:    toDir,
			}); err != nil {
				return err
			}
			fmt.Printf("done.\n")

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
