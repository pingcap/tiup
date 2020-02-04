package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/AstroProfundis/tiup-demo/tiup/pkg/utils"
	"github.com/spf13/cobra"
)

type showCmd struct {
	*baseCmd
}

func newShowCmd() *showCmd {
	var (
		showAll bool
	)

	cmdShow := &showCmd{
		newBaseCmd(&cobra.Command{
			Use:   "show",
			Short: "Show the available TiDB components",
			Long:  `Show available and installed TiDB components and their versions.`,
			RunE: func(cmd *cobra.Command, args []string) error {
				if showAll {
					return showOnlineComponentList()
				}
				fmt.Println("TODO: show installed list...")
				return nil
			},
		}),
	}

	cmdShow.cmd.Flags().BoolVar(&showAll, "all", false, "Show all available components and versions (refresh online).")

	return cmdShow
}

func showOnlineComponentList() error {
	onlineList, err := fetchComponentList(componentListURL)
	if err != nil {
		return err
	}
	for _, comp := range onlineList.Components {
		fmt.Println("Available components:")
		var cmpTable [][]string
		cmpTable = append(cmpTable, []string{"Name", "Version", "Description"})
		for _, ver := range comp.VersionList {
			cmpTable = append(cmpTable, []string{comp.Name, ver.Version, comp.Description})
		}
		utils.PrintTable(cmpTable, true)
	}
	return nil
}

type compVer struct {
	Version string `json:"version,omitempty"`
	SHA256  string `json:"sha256,omitempty"`
	URL     string `json:"url,omitempty"`
}

type compItem struct {
	Name        string    `json:"name,omitempty"`
	VersionList []compVer `json:"versions,omitempty"`
	Description string    `json:"description,omitempty"`
}

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

// decodeComponentList decode the http response data to a JSON object
func decodeComponentList(resp *http.Response) (*compMeta, error) {
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var body compMeta
	err = json.Unmarshal(bodyBytes, &body)
	if err != nil {
		return nil, err
	}

	return &body, nil
}
