// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
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
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/pingcap-incubator/tiup-cluster/pkg/log"
	"github.com/pingcap-incubator/tiup-cluster/pkg/utils"
	"github.com/pingcap/errors"
	"github.com/pingcap/kvproto/pkg/metapb"
	"github.com/pingcap/kvproto/pkg/pdpb"
	pdserverapi "github.com/pingcap/pd/v4/server/api"
)

// PDClient is an HTTP client of the PD server
type PDClient struct {
	addrs      []string
	tlsEnabled bool
	httpClient *utils.HTTPClient
}

// NewPDClient returns a new PDClient
func NewPDClient(addrs []string, timeout time.Duration, tlsConfig *tls.Config) *PDClient {
	enableTLS := false
	if tlsConfig != nil {
		enableTLS = true
	}

	return &PDClient{
		addrs:      addrs,
		tlsEnabled: enableTLS,
		httpClient: utils.NewHTTPClient(timeout, tlsConfig),
	}
}

// GetURL builds the the client URL of PDClient
func (pc *PDClient) GetURL(addr string) string {
	httpPrefix := "http"
	if pc.tlsEnabled {
		httpPrefix = "https"
	}
	return fmt.Sprintf("%s://%s", httpPrefix, addr)
}

// nolint (some is unused now)
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
	pdConfigReplicate   = "pd/api/v1/config/replicate"
)

type doFunc func(endpoint string) error
type getFunc func(endpoint string) ([]byte, error)

func tryURLs(endpoints []string, f doFunc) error {
	var err error
	for _, endpoint := range endpoints {
		var u *url.URL
		u, err = url.Parse(endpoint)

		if err != nil {
			return errors.AddStack(err)
		}

		endpoint = u.String()

		err = f(endpoint)
		if err != nil {
			continue
		}
		break
	}
	if len(endpoints) > 1 && err != nil {
		err = errors.Errorf("after trying all endpoints, no endpoint is available, the last error we met: %s", err)
	}
	return err
}

func tryURLsGet(endpoints []string, f getFunc) ([]byte, error) {
	var err error
	var bytes []byte
	for _, endpoint := range endpoints {
		var u *url.URL
		u, err = url.Parse(endpoint)

		if err != nil {
			return nil, errors.AddStack(err)
		}

		endpoint = u.String()

		bytes, err = f(endpoint)
		if err != nil {
			continue
		}
		break
	}
	if len(endpoints) > 1 && err != nil {
		return nil, errors.Errorf("after trying all endpoints, no endpoint is available, the last error we met: %s", err)
	}
	return bytes, err
}

// PDHealthInfo is the member health info from PD's API
type PDHealthInfo struct {
	Healths []pdserverapi.Health
}

func (pc *PDClient) getEndpoints(cmd string) (endpoints []string) {
	for _, addr := range pc.addrs {
		endpoint := fmt.Sprintf("%s/%s", pc.GetURL(addr), cmd)
		endpoints = append(endpoints, endpoint)
	}

	return
}

// GetHealth queries the health info from PD server
func (pc *PDClient) GetHealth() (*PDHealthInfo, error) {
	endpoints := pc.getEndpoints(pdHealthURI)

	healths := []pdserverapi.Health{}

	err := tryURLs(endpoints, func(endpoint string) error {
		body, err := pc.httpClient.Get(endpoint)
		if err != nil {
			return err
		}

		return json.Unmarshal(body, &healths)
	})

	if err != nil {
		return nil, errors.AddStack(err)
	}

	return &PDHealthInfo{healths}, nil
}

// GetStores queries the stores info from PD server
func (pc *PDClient) GetStores() (*pdserverapi.StoresInfo, error) {
	// Return all stores
	query := "?state=0&state=1&state=2"
	endpoints := pc.getEndpoints(pdStoresURI + query)

	storesInfo := pdserverapi.StoresInfo{}

	err := tryURLs(endpoints, func(endpoint string) error {
		body, err := pc.httpClient.Get(endpoint)
		if err != nil {
			return err
		}

		return json.Unmarshal(body, &storesInfo)

	})
	if err != nil {
		return nil, errors.AddStack(err)
	}

	return &storesInfo, nil
}

// WaitLeader wait until there's a leader or timeout.
func (pc *PDClient) WaitLeader(retryOpt *utils.RetryOption) error {
	if retryOpt == nil {
		retryOpt = &utils.RetryOption{
			Delay:   time.Second * 1,
			Timeout: time.Second * 30,
		}
	}

	if err := utils.Retry(func() error {
		_, err := pc.GetLeader()
		if err == nil {
			return nil
		}

		// return error by default, to make the retry work
		log.Debugf("Still waitting for the PD leader to be elected")
		return errors.New("still waitting for the PD leader to be elected")
	}, *retryOpt); err != nil {
		return fmt.Errorf("error getting PD leader, %v", err)
	}
	return nil
}

// GetLeader queries the leader node of PD cluster
func (pc *PDClient) GetLeader() (*pdpb.Member, error) {
	endpoints := pc.getEndpoints(pdLeaderURI)

	leader := pdpb.Member{}

	err := tryURLs(endpoints, func(endpoint string) error {
		body, err := pc.httpClient.Get(endpoint)
		if err != nil {
			return err
		}

		return json.Unmarshal(body, &leader)
	})

	if err != nil {
		return nil, errors.AddStack(err)
	}

	return &leader, nil
}

// GetMembers queries for member list from the PD server
func (pc *PDClient) GetMembers() (*pdpb.GetMembersResponse, error) {
	endpoints := pc.getEndpoints(pdMembersURI)
	members := pdpb.GetMembersResponse{}

	err := tryURLs(endpoints, func(endpoint string) error {
		body, err := pc.httpClient.Get(endpoint)
		if err != nil {
			return err
		}

		return json.Unmarshal(body, &members)
	})

	if err != nil {
		return nil, errors.AddStack(err)
	}

	return &members, nil
}

// EvictPDLeader evicts the PD leader
func (pc *PDClient) EvictPDLeader(retryOpt *utils.RetryOption) error {
	// get current members
	members, err := pc.GetMembers()
	if err != nil {
		return err
	}

	if len(members.Members) == 1 {
		log.Warnf("Only 1 member in the PD cluster, skip leader evicting")
		return nil
	}

	// try to evict the leader
	cmd := fmt.Sprintf("%s/resign", pdLeaderURI)
	endpoints := pc.getEndpoints(cmd)

	err = tryURLs(endpoints, func(endpoint string) error {
		_, err = pc.httpClient.Post(endpoint, nil)
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return errors.AddStack(err)
	}

	// wait for the transfer to complete
	if retryOpt == nil {
		retryOpt = &utils.RetryOption{
			Delay:   time.Second * 5,
			Timeout: time.Second * 300,
		}
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
		log.Debugf("Still waitting for the PD leader to transfer")
		return errors.New("still waitting for the PD leader to transfer")
	}, *retryOpt); err != nil {
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
func (pc *PDClient) EvictStoreLeader(host string, retryOpt *utils.RetryOption) error {
	// get info of current stores
	stores, err := pc.GetStores()
	if err != nil {
		return err
	}

	// get store info of host
	var latestStore *pdserverapi.StoreInfo
	for _, storeInfo := range stores.Stores {
		if storeInfo.Store.Address != host {
			continue
		}
		if latestStore == nil {
			latestStore = storeInfo
			continue
		}
		if storeInfo.Store.Id > latestStore.Store.Id {
			latestStore = storeInfo
		}
	}

	if latestStore == nil || latestStore.Status.LeaderCount == 0 {
		// no store leader on the host, just skip
		return nil
	}

	log.Infof("Evicting %d leaders from store %s...",
		latestStore.Status.LeaderCount, latestStore.Store.Address)

	// set scheduler for stores
	scheduler, err := json.Marshal(pdSchedulerRequest{
		Name:    pdEvictLeaderName,
		StoreID: latestStore.Store.Id,
	})
	if err != nil {
		return nil
	}

	endpoints := pc.getEndpoints(pdSchedulersURI)

	err = tryURLs(endpoints, func(endpoint string) error {
		_, err := pc.httpClient.Post(endpoint, bytes.NewBuffer(scheduler))
		return err
	})
	if err != nil {
		return errors.AddStack(err)
	}

	// wait for the transfer to complete
	if retryOpt == nil {
		retryOpt = &utils.RetryOption{
			Delay:   time.Second * 5,
			Timeout: time.Second * 600,
		}
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
			log.Debugf(
				"Still waitting for %d store leaders to transfer...",
				currStoreInfo.Status.LeaderCount,
			)
		}

		// return error by default, to make the retry work
		return errors.New("still waiting for the store leaders to transfer")
	}, *retryOpt); err != nil {
		return fmt.Errorf("error evicting store leader from %s, %v", host, err)
	}
	return nil
}

// RemoveStoreEvict removes a store leader evict scheduler, which allows following
// leaders to be transffered to it again.
func (pc *PDClient) RemoveStoreEvict(host string) error {
	// get info of current stores
	stores, err := pc.GetStores()
	if err != nil {
		return err
	}

	// get store info of host
	var latestStore *pdserverapi.StoreInfo
	for _, storeInfo := range stores.Stores {
		if storeInfo.Store.Address != host {
			continue
		}
		if latestStore == nil {
			latestStore = storeInfo
			continue
		}
		if storeInfo.Store.Id > latestStore.Store.Id {
			latestStore = storeInfo
		}
	}

	if latestStore == nil {
		// no store matches, just skip
		return nil
	}

	// remove scheduler for the store
	cmd := fmt.Sprintf(
		"%s/%s",
		pdSchedulersURI,
		fmt.Sprintf("%s-%d", pdEvictLeaderName, latestStore.Store.Id),
	)
	endpoints := pc.getEndpoints(cmd)

	err = tryURLs(endpoints, func(endpoint string) error {
		body, statusCode, err := pc.httpClient.Delete(endpoint, nil)
		if err != nil {
			if statusCode == 404 || bytes.Contains(body, []byte("scheduler not found")) {
				log.Debugf("Store leader evicting scheduler does not exist, ignore.")
				return nil
			}
			return err
		}
		log.Debugf("Delete leader evicting scheduler of store %d success", latestStore.Store.Id)
		return nil
	})
	if err != nil {
		return errors.AddStack(err)
	}

	log.Debugf("Removed store leader evicting scheduler from %s.", latestStore.Store.Address)
	return nil
}

// DelPD deletes a PD node from the cluster, name is the Name of the PD member
func (pc *PDClient) DelPD(name string, retryOpt *utils.RetryOption) error {
	// get current members
	members, err := pc.GetMembers()
	if err != nil {
		return err
	}
	if len(members.Members) == 1 {
		return errors.New("at least 1 PD node must be online, can not delete")
	}

	// try to delete the node
	cmd := fmt.Sprintf("%s/name/%s", pdMembersURI, name)
	endpoints := pc.getEndpoints(cmd)

	err = tryURLs(endpoints, func(endpoint string) error {
		_, _, err := pc.httpClient.Delete(endpoint, nil)
		return err
	})
	if err != nil {
		return errors.AddStack(err)
	}

	// wait for the deletion to complete
	if retryOpt == nil {
		retryOpt = &utils.RetryOption{
			Delay:   time.Second * 2,
			Timeout: time.Second * 60,
		}
	}
	if err := utils.Retry(func() error {
		currMembers, err := pc.GetMembers()
		if err != nil {
			return err
		}

		// check if the deleted member still present
		for _, member := range currMembers.Members {
			if member.Name == name {
				return errors.New("still waitting for the PD node to be deleted")
			}
		}

		return nil
	}, *retryOpt); err != nil {
		return fmt.Errorf("error deleting PD node, %v", err)
	}
	return nil
}

// IsTombStone check if the node is Tombstone.
// The host parameter should be in format of IP:Port, that matches store's address
func (pc *PDClient) IsTombStone(host string) (bool, error) {
	// get info of current stores
	stores, err := pc.GetStores()
	if err != nil {
		return false, errors.AddStack(err)
	}

	for _, storeInfo := range stores.Stores {
		// log.Debugf("host: %s addr: %s, state: %v", host, storeInfo.Store.Address, storeInfo.Store.State)

		if storeInfo.Store.Address != host {
			continue
		}

		if storeInfo.Store.State == metapb.StoreState_Tombstone {
			return true, nil
		}
		return false, nil

	}

	return false, errors.New("node not exists")
}

// ErrStoreNotExists represents the store not exists.
var ErrStoreNotExists = errors.New("store not exists")

// DelStore deletes stores from a (TiKV) host
// The host parameter should be in format of IP:Port, that matches store's address
func (pc *PDClient) DelStore(host string, retryOpt *utils.RetryOption) error {
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
	}
	if storeID == 0 {
		return errors.Annotatef(ErrStoreNotExists, "id: %s", host)
	}

	cmd := fmt.Sprintf("%s/%d", pdStoreURI, storeID)
	endpoints := pc.getEndpoints(cmd)

	err = tryURLs(endpoints, func(endpoint string) error {
		_, _, err := pc.httpClient.Delete(endpoint, nil)
		return err
	})
	if err != nil {
		return errors.AddStack(err)
	}

	// wait for the deletion to complete
	if retryOpt == nil {
		retryOpt = &utils.RetryOption{
			Delay:   time.Second * 2,
			Timeout: time.Second * 60,
		}
	}
	if err := utils.Retry(func() error {
		currStores, err := pc.GetStores()
		if err != nil {
			return err
		}

		// check if the deleted member still present
		for _, store := range currStores.Stores {
			if store.Store.Id == storeID {
				// deleting a store may take long time to transfer data, so we
				// return success once it get to "Offline" status and not waiting
				// for the whole process to complete.
				// When finished, the store's state will be "Tombstone".
				if store.Store.StateName != metapb.StoreState_name[0] {
					return nil
				}
				return errors.New("still waiting for the store to be deleted")
			}
		}

		return nil
	}, *retryOpt); err != nil {
		return fmt.Errorf("error deleting store, %v", err)
	}
	return nil
}

// UpdateReplicateConfig updates the PD replicate config
func (pc *PDClient) UpdateReplicateConfig(body io.Reader) error {
	endpoints := pc.getEndpoints(pdConfigReplicate)
	return tryURLs(endpoints, func(endpoint string) error {
		_, err := pc.httpClient.Post(endpoint, body)
		if err != nil {
			return err
		}
		return nil
	})
}

// GetReplicateConfig gets the PD replicate config
func (pc *PDClient) GetReplicateConfig() ([]byte, error) {
	endpoints := pc.getEndpoints(pdConfigReplicate)
	return tryURLsGet(endpoints, func(endpoint string) ([]byte, error) {
		ret, err := pc.httpClient.Get(endpoint)
		if err != nil {
			return nil, err
		}
		return ret, nil
	})
}
