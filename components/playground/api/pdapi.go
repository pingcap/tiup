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
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/pingcap-incubator/tiup/components/playground/utils"
	"github.com/pingcap/errors"
	"github.com/pingcap/kvproto/pkg/metapb"
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

var (
	pdStoresURI       = "pd/api/v1/stores"
	pdStoreURI        = "pd/api/v1/store"
	pdConfigReplicate = "pd/api/v1/config/replicate"
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

func (pc *PDClient) getEndpoints(cmd string) (endpoints []string) {
	for _, addr := range pc.addrs {
		endpoint := fmt.Sprintf("%s/%s", pc.GetURL(addr), cmd)
		endpoints = append(endpoints, endpoint)
	}

	return
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

// IsTombStone check if the node is Tombstone.
// The host parameter should be in format of IP:Port, that matches store's address
func (pc *PDClient) IsTombStone(host string) (bool, error) {
	// get info of current stores
	stores, err := pc.GetStores()
	if err != nil {
		return false, errors.AddStack(err)
	}

	for _, storeInfo := range stores.Stores {

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
