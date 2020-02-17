package meta

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/AstroProfundis/tiup-demo/tiup/pkg/utils"
)

var (
	componentFileName = "components.json"
)

// CompVer is the version metadata of a component
type CompVer struct {
	Version string `json:"version,omitempty"`
	SHA256  string `json:"sha256,omitempty"`
	Size    uint64 `json:"size,omitempty"`
	URL     string `json:"url,omitempty"`
}

// CompItem is the metadata of a component
type CompItem struct {
	Name        string    `json:"name,omitempty"`
	VersionList []CompVer `json:"versions,omitempty"`
	Description string    `json:"description,omitempty"`
}

// CompMeta is the component metadata list
type CompMeta struct {
	Components  []CompItem `json:"components,omitempty"`
	Description string     `json:"description,omitempty"`
	Modified    time.Time  `json:"modified,omitempty"`
}

// FetchComponentList request and get latest component metadata list online
func FetchComponentList(url string) (*CompMeta, error) {
	fmt.Println("Fetching latest component list online...")
	resp, err := utils.NewClient(url, nil).Get()
	if err != nil {
		fmt.Println("Error fetching component list.")
		return nil, err
	}
	return decodeComponentList(resp)
}

// SaveComponentList saves the component metadata list to disk
func SaveComponentList(comp *CompMeta) error {
	return utils.WriteJSON(componentFileName, comp)
}

// ReadComponentList reads component metadata list from disk
func ReadComponentList() (*CompMeta, error) {
	data, err := utils.ReadFile(componentFileName)
	if err != nil {
		return nil, err
	}

	return unmarshalComponentList(data)
}

// decodeComponentList decode the http response data to a JSON object
func decodeComponentList(resp *http.Response) (*CompMeta, error) {
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return unmarshalComponentList(bodyBytes)
}

func unmarshalComponentList(data []byte) (*CompMeta, error) {
	var meta CompMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}
