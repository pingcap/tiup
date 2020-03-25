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
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/pingcap-incubator/tiops/pkg/utils"
	"github.com/pingcap/kvproto/pkg/pdpb"
	pdserverapi "github.com/pingcap/pd/v4/server/api"
)

// PDClient is an HTTP client of the PD server "Host"
type PDClient struct {
	Host       string
	tlsEnabled bool
	httpClient *utils.HTTPClient
}

// NewPDClient returns a new PDClient
func NewPDClient(host string, timeout time.Duration, tlsConfig *tls.Config) *PDClient {
	enableTLS := false
	if tlsConfig != nil {
		enableTLS = true
	}

	return &PDClient{
		Host:       host,
		tlsEnabled: enableTLS,
		httpClient: utils.NewHTTPClient(timeout, tlsConfig),
	}
}

// GetURL builds the the client URL of PDClient
func (pc *PDClient) GetURL() string {
	httpPrefix := "http"
	if pc.tlsEnabled {
		httpPrefix = "https"
	}
	return fmt.Sprintf("%s://%s", httpPrefix, pc.Host)
}

var (
	pdHealthURI         = "pd/health"
	pdMembersURI        = "pd/api/v1/members"
	pdStoresURI         = "pd/api/v1/stores"
	pdStoreURI          = "pd/api/v1/store"
	pdConfigURI         = "pd/api/v1/config"
	pdClusterIDURI      = "pd/api/v1/cluster"
	pdSchedulersURI     = "pd/api/v1/schedulers"
	pdLeaderURI         = "pd/api/v1/leader"
	pdLeaderTransferURI = "pd/api/v1/leader/transfer"
)

// PDHealthInfo is the member health info from PD's API
type PDHealthInfo struct {
	Healths []pdserverapi.Health
}

// GetHealth queries the health info from PD server
func (pc *PDClient) GetHealth() (*PDHealthInfo, error) {
	url := fmt.Sprintf("%s/%s", pc.GetURL(), pdHealthURI)
	body, err := pc.httpClient.Get(url)
	if err != nil {
		return nil, err
	}

	healths := []pdserverapi.Health{}
	if err := json.Unmarshal(body, &healths); err != nil {
		return nil, err
	}
	return &PDHealthInfo{healths}, nil
}

// GetStores queries the stores info from PD server
func (pc *PDClient) GetStores() (*pdserverapi.StoresInfo, error) {
	url := fmt.Sprintf("%s/%s", pc.GetURL(), pdStoresURI)
	body, err := pc.httpClient.Get(url)
	if err != nil {
		return nil, err
	}

	storesInfo := pdserverapi.StoresInfo{}
	if err := json.Unmarshal(body, &storesInfo); err != nil {
		return nil, err
	}
	return &storesInfo, nil
}

// GetLeader queries the leader node of PD cluster
func (pc *PDClient) GetLeader() (*pdpb.Member, error) {
	url := fmt.Sprintf("%s/%s", pc.GetURL(), pdLeaderURI)
	body, err := pc.httpClient.Get(url)
	if err != nil {
		return nil, err
	}

	leader := pdpb.Member{}
	if err := json.Unmarshal(body, &leader); err != nil {
		return nil, err
	}
	return &leader, nil
}

// GetMembers queries for member list from the PD server
func (pc *PDClient) GetMembers() (*pdpb.GetMembersResponse, error) {
	url := fmt.Sprintf("%s/%s", pc.GetURL(), pdMembersURI)
	body, err := pc.httpClient.Get(url)
	if err != nil {
		return nil, err
	}

	members := pdpb.GetMembersResponse{}
	if err := json.Unmarshal(body, &members); err != nil {
		return nil, err
	}
	return &members, nil
}

// EvictPDLeader evicts the PD leader
func (pc *PDClient) EvictPDLeader() error {
	// get current members
	members, err := pc.GetMembers()
	if err != nil {
		return err
	}
	if len(members.Members) == 1 {
		// TODO: add a warning log here say:
		// "Only 1 member in the PD cluster, skip leader evicting"
		return nil
	}

	// try to evict the leader
	url := fmt.Sprintf("%s/%s/resign", pc.GetURL(), pdLeaderURI)
	_, err = pc.httpClient.Post(url, bytes.NewBuffer([]byte("")))
	if err != nil {
		return err
	}

	// wait for the transfer to complete
	retryOpt := utils.RetryOption{
		Attempts: 60,
		Delay:    time.Second * 5,
		Timeout:  time.Second * 300,
	}
	if err := utils.Retry(func() error {
		currLeader, err := pc.GetLeader()
		if err != nil {
			return err
		}

		// check if current leader is the leader to evict
		if currLeader.Name != members.Leader.Name {
			return nil
		}

		// return error by default, to make the retry work
		return errors.New("still waitting for the PD leader to transfer")
	}, retryOpt); err != nil {
		return fmt.Errorf("error evicting PD leader, %v", err)
	}
	return nil
}

const (
	// pdEvictLeaderName is evict leader scheduler name.
	pdEvictLeaderName = "evict-leader-scheduler"
)

// pdSchedulerRequest is the request body when evicting store leader
type pdSchedulerRequest struct {
	Name    string `json:"name"`
	StoreID uint64 `json:"store_id"`
}

// EvictStoreLeader evicts the store leaders
// The host parameter should be in format of IP:Port, that matches store's address
func (pc *PDClient) EvictStoreLeader(host string) error {
	// get info of current stores
	stores, err := pc.GetStores()
	if err != nil {
		return err
	}

	// get store ID of host
	var storeID uint64
	for _, storeInfo := range stores.Stores {
		if storeInfo.Store.Address != host {
			continue
		}
		storeID = storeInfo.Store.Id

		if storeInfo.Status.LeaderCount == 0 {
			// no store leader on the host, just skip
			return nil
		}
		// TODO: add a log say
		// Evicting leader from storeInfo.Status.LeaderCount stores
	}

	// set scheduler for stores
	scheduler, err := json.Marshal(pdSchedulerRequest{
		Name:    pdEvictLeaderName,
		StoreID: storeID,
	})
	if err != nil {
		return nil
	}
	url := fmt.Sprintf("%s/%s", pc.GetURL(), pdSchedulersURI)
	_, err = pc.httpClient.Post(url, bytes.NewBuffer(scheduler))
	if err != nil {
		return err
	}

	// wait for the transfer to complete
	retryOpt := utils.RetryOption{
		Attempts: 72,
		Delay:    time.Second * 5,
		Timeout:  time.Second * 360,
	}
	if err := utils.Retry(func() error {
		currStores, err := pc.GetStores()
		if err != nil {
			return err
		}

		// check if all leaders are evicted
		for _, currStoreInfo := range currStores.Stores {
			if currStoreInfo.Store.Address != host {
				continue
			}
			if currStoreInfo.Status.LeaderCount == 0 {
				return nil
			}
		}

		// return error by default, to make the retry work
		return errors.New("still waitting for the store leaders to transfer")
	}, retryOpt); err != nil {
		return fmt.Errorf("error evicting store leader from %s, %v", host, err)
	}
	return nil
}
