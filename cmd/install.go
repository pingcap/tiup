package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/c4pt0r/tiup/pkg/meta"
	"github.com/c4pt0r/tiup/pkg/utils"
	"github.com/spf13/cobra"
)

const (
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
				var err error
				switch argsLen {
				case 0:
					cmd.Help()
					return nil
				case 1: // version unspecified, use stable latest as default
					currChan, err := meta.ReadVersionFile()
					if os.IsNotExist(err) {
						fmt.Println("default version not set, using latest stable.")
						compMeta, err := meta.ReadComponentList()
						if os.IsNotExist(err) {
							fmt.Println("no available component list, try `tiup component list --refresh` to get latest online list.")
							return nil
						} else if err != nil {
							return err
						}
						version = compMeta.Stable
					} else if err != nil {
						return err
					}
					version = currChan.Ver
					componentList = args
				default:
					version, err = utils.FmtVer(args[argsLen-1])
					if err != nil {
						return err
					}
					componentList = args[:argsLen-1]
				}
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
	meta, err := meta.ReadComponentList()
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

			// decompress files to a temp dir, and try to keep it unique
			tmpDir := utils.MustDir(path.Join(profileDir, "tmp/", checksum))
			fmt.Printf("Decompressing...")
			if err = utils.Untar(tarball, tmpDir); err != nil {
				return err
			}

			// move binaries to final path
			tmpBin := path.Join(tmpDir,
				strings.TrimSuffix(filepath.Base(tarball), ".tar.gz"),
				"bin")
			toDir = path.Join(
				utils.MustDir(path.Join(profileDir, ver)),
				comp)
			if err := utils.Rename(tmpBin, toDir); err != nil {
				return err
			}
			// remove the temp dir (should be empty)
			if err := os.RemoveAll(tmpDir); err != nil {
				fmt.Printf("fail to remove temp directory %s\n", tmpDir)
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

func getComponentURL(list []meta.CompItem, ver string, comp string) (string, string) {
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
