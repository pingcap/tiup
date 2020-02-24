package meta

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/c4pt0r/tiup/pkg/utils"
)

const (
	componentFileName = "components.json"
)

// CompVer is the version metadata of a component
type CompVer struct {
	Version string `json:"version,omitempty"`
	SHA256  string `json:"sha256,omitempty"` // checksum of the tarball
	Size    uint64 `json:"size,omitempty"`   // size of the tarball in bytes
	URL     string `json:"url,omitempty"`    // URL to the tarball
	OS      string `json:"os,omitempty"`     // OS matching the binary
	Arch    string `json:"arch,omitempty"`   // Arch matching the binary
}

// CompItem is the metadata of a component
type CompItem struct {
	Name        string    `json:"name,omitempty"`
	Description string    `json:"description,omitempty"`
	VersionList []CompVer `json:"versions,omitempty"` // list of available versions
}

// CompMeta is the component metadata list
type CompMeta struct {
	Components  []CompItem `json:"components,omitempty"`
	Description string     `json:"description,omitempty"`
	Modified    time.Time  `json:"modified,omitempty"`
	Stable      string     `json:"stable,omitempty"` // the latest GA version
	Beta        string     `json:"beta,omitempty"`   // the latest Beta version
	// TODO: nightly should have a special config, using something like "-nightly"
	// as version and store the date on disk, so only one URL is needed for the
	// nightly channel
	//Nightly     string    `json:"nightly,omitempty"`  // the last nightly version

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
