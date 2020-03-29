// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package api

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/pingcap/errors"
)

var (
	pumpStatusURI    = "/status"
	drainerStatusURI = "/drainers"
	binlogStateURI   = "/state"
)

// BinlogClient is the client of binlog.
type BinlogClient struct {
	tls        *tls.Config
	httpClient *http.Client
}

// NewBinlogClient create a BinlogClient.
func NewBinlogClient(tlsConfig *tls.Config) *BinlogClient {
	return &BinlogClient{
		tls: tlsConfig,
		httpClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		},
	}
}

func (c *BinlogClient) getURL(addr string) string {
	schema := "http"
	if c.tls != nil {
		schema = "https"
	}

	return fmt.Sprintf("%s://%s", schema, addr)
}

func (c *BinlogClient) getOfflineURL(addr string, nodeID string) string {
	return fmt.Sprintf("%s/state/%s/close", c.getURL(addr), nodeID)
}

// StatusResp represents the response of status api.
type StatusResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (c *BinlogClient) offline(addr string, nodeID string) error {
	url := c.getOfflineURL(addr, nodeID)
	req, err := http.NewRequest("PUT", url, nil)
	if err != nil {
		return errors.AddStack(err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return errors.AddStack(err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return errors.Errorf("error requesting %s, code: %d",
			resp.Request.URL, resp.StatusCode)
	}

	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)

	var status StatusResp
	err = json.Unmarshal(data, &status)
	if err != nil {
		return errors.Annotatef(err, "data: %s", string(data))
	}

	if status.Code != 200 {
		return errors.Errorf("server error: %s", status.Message)
	}

	return nil
}

// OfflinePump offline a pump.
func (c *BinlogClient) OfflinePump(addr string, nodeID string) error {
	return c.offline(addr, nodeID)
}

// OfflineDrainer offline a drainer.
func (c *BinlogClient) OfflineDrainer(addr string, nodeID string) error {
	return c.offline(addr, nodeID)
}
