package cmd

import (
	"fmt"
	"os"

	"code.cloudfoundry.org/bytefmt"
	"github.com/c4pt0r/tiup/pkg/meta"
	"github.com/c4pt0r/tiup/pkg/utils"
	"github.com/spf13/cobra"
)

var (
	componentListURL = "https://repo.hoshi.at/tmp/components.json"
)

type listComponentCmd struct {
	*baseCmd
}

func newListComponentCmd() *listComponentCmd {
	var (
		showAll bool
		refresh bool
	)

	cmdListComponent := &listComponentCmd{
		newBaseCmd(&cobra.Command{
			Use:   "list",
			Short: "List the available TiDB components",
			Long:  `List available and installed TiDB components and their versions.`,
			RunE: func(cmd *cobra.Command, args []string) error {
				if refresh {
					compList, err := meta.FetchComponentList(componentListURL)
					if err != nil {
						return err
					}
					// save latest component list to local
					if err := meta.SaveComponentList(compList); err != nil {
						return err
					}
					showComponentList(compList)
					return nil
				}

				if showAll {
					compList, err := meta.ReadComponentList()
					if err != nil {
						if os.IsNotExist(err) {
							fmt.Println("no available component list, try `tiup component list --refresh` to get latest online list.")
							return nil
						}
						return err
					}
					showComponentList(compList)
				} else {
					return showInstalledList()
				}
				return nil
			},
		}),
	}

	cmdListComponent.cmd.Flags().BoolVar(&showAll, "all", false, "List all available components and versions (from local cache).")
	cmdListComponent.cmd.Flags().BoolVar(&refresh, "refresh", false, "Refresh online list of components and versions.")

	return cmdListComponent
}

func showComponentList(compList *meta.CompMeta) {
	for _, comp := range compList.Components {
		fmt.Println("Available components:")
		fmt.Printf("(%s)\n", comp.Description)
		var cmpTable [][]string
		cmpTable = append(cmpTable, []string{"Name", "Version", "Size", "Installed"})
		for _, ver := range comp.VersionList {
			installStatus := ""
			installed, err := checkInstalledComponent(comp.Name, ver.Version)
			if err != nil {
				fmt.Printf("Unable to check for installed components: %s\n", err)
				return
			}
			if installed {
				installStatus = "yes"
			}
			cmpTable = append(cmpTable, []string{
				comp.Name,
				ver.Version,
				bytefmt.ByteSize(ver.Size),
				installStatus,
			})
		}
		utils.PrintTable(cmpTable, true)
	}
}

func showInstalledList() error {
	list, err := getInstalledList()
	if err != nil {
		return err
	}

	if len(list) < 1 {
		fmt.Println("no installed component, try `tiup component list --all` to see all available components.")
		return nil
	}

	fmt.Println("Installed components:")
	var instTable [][]string
	instTable = append(instTable, []string{"Name", "Version", "Path"})
	for _, item := range list {
		instTable = append(instTable, []string{
			item.Name,
			item.Version,
			item.Path,
		})
	}

	utils.PrintTable(instTable, true)
	return nil
}
