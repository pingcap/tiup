package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"code.cloudfoundry.org/bytefmt"
	"github.com/AstroProfundis/tiup-demo/tiup/pkg/utils"
	"github.com/spf13/cobra"
)

var (
	componentFileName = "components.json"
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
					compList, err := fetchComponentList(componentListURL)
					if err != nil {
						return err
					}
					// save latest component list to local
					if err := saveComponentList(compList); err != nil {
						return err
					}
					showComponentList(compList)
					return nil
				}

				if showAll {
					compList, err := readComponentList()
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

func showComponentList(compList *compMeta) {
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

type compVer struct {
	Version string `json:"version,omitempty"`
	SHA256  string `json:"sha256,omitempty"`
	Size    uint64 `json:"size,omitempty"`
	URL     string `json:"url,omitempty"`
}

type compItem struct {
	Name        string    `json:"name,omitempty"`
	VersionList []compVer `json:"versions,omitempty"`
	Description string    `json:"description,omitempty"`
}

// Component meta list
type compMeta struct {
	Components  []compItem `json:"components,omitempty"`
	Description string     `json:"description,omitempty"`
	Modified    time.Time  `json:"modified,omitempty"`
}

func fetchComponentList(url string) (*compMeta, error) {
	fmt.Println("Fetching latest component list online...")
	resp, err := utils.NewClient(url, nil).Get()
	if err != nil {
		fmt.Println("Error fetching component list.")
		return nil, err
	}
	return decodeComponentList(resp)
}

func saveComponentList(comp *compMeta) error {
	return utils.WriteJSON(componentFileName, comp)
}

func readComponentList() (*compMeta, error) {
	data, err := utils.ReadFile(componentFileName)
	if err != nil {
		return nil, err
	}

	return unmarshalComponentList(data)
}

// decodeComponentList decode the http response data to a JSON object
func decodeComponentList(resp *http.Response) (*compMeta, error) {
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return unmarshalComponentList(bodyBytes)
}

func unmarshalComponentList(data []byte) (*compMeta, error) {
	var meta compMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}
