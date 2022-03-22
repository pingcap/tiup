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
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jeremywohl/flatten"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/kvproto/pkg/metapb"
	"github.com/pingcap/kvproto/pkg/pdpb"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/utils"
	"golang.org/x/mod/semver"
)

// PDClient is an HTTP client of the PD server
type PDClient struct {
	version    string
	addrs      []string
	tlsEnabled bool
	httpClient *utils.HTTPClient
	ctx        context.Context
}

// LabelInfo represents an instance label info
type LabelInfo struct {
	Machine   string `json:"machine"`
	Port      string `json:"port"`
	Store     uint64 `json:"store"`
	Status    string `json:"status"`
	Leaders   int    `json:"leaders"`
	Regions   int    `json:"regions"`
	Capacity  string `json:"capacity"`
	Available string `json:"available"`
	Labels    string `json:"labels"`
}

// NewPDClient returns a new PDClient, the context must have
// a *logprinter.Logger as value of "logger"
func NewPDClient(
	ctx context.Context,
	addrs []string,
	timeout time.Duration,
	tlsConfig *tls.Config,
) *PDClient {
	enableTLS := false
	if tlsConfig != nil {
		enableTLS = true
	}

	if _, ok := ctx.Value(logprinter.ContextKeyLogger).(*logprinter.Logger); !ok {
		panic("the context must have logger inside")
	}

	cli := &PDClient{
		addrs:      addrs,
		tlsEnabled: enableTLS,
		httpClient: utils.NewHTTPClient(timeout, tlsConfig),
		ctx:        ctx,
	}

	cli.tryIdentifyVersion()
	return cli
}

func (pc *PDClient) l() *logprinter.Logger {
	return pc.ctx.Value(logprinter.ContextKeyLogger).(*logprinter.Logger)
}

func (pc *PDClient) tryIdentifyVersion() {
	endpoints := pc.getEndpoints(pdVersionURI)
	response := map[string]string{}
	_, err := tryURLs(endpoints, func(endpoint string) ([]byte, error) {
		body, err := pc.httpClient.Get(pc.ctx, endpoint)
		if err != nil {
			return body, err
		}

		return body, json.Unmarshal(body, &response)
	})
	if err == nil {
		pc.version = response["version"]
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

const (
	// pdEvictLeaderName is evict leader scheduler name.
	pdEvictLeaderName = "evict-leader-scheduler"
)

// nolint (some is unused now)
var (
	pdPingURI            = "pd/ping"
	pdVersionURI         = "pd/api/v1/version"
	pdConfigURI          = "pd/api/v1/config"
	pdClusterIDURI       = "pd/api/v1/cluster"
	pdConfigReplicate    = "pd/api/v1/config/replicate"
	pdReplicationModeURI = "pd/api/v1/config/replication-mode"
	pdRulesURI           = "pd/api/v1/config/rules"
	pdConfigSchedule     = "pd/api/v1/config/schedule"
	pdLeaderURI          = "pd/api/v1/leader"
	pdLeaderTransferURI  = "pd/api/v1/leader/transfer"
	pdMembersURI         = "pd/api/v1/members"
	pdSchedulersURI      = "pd/api/v1/schedulers"
	pdStoreURI           = "pd/api/v1/store"
	pdStoresURI          = "pd/api/v1/stores"
	pdStoresLimitURI     = "pd/api/v1/stores/limit"
	pdRegionsCheckURI    = "pd/api/v1/regions/check"
)

func tryURLs(endpoints []string, f func(endpoint string) ([]byte, error)) ([]byte, error) {
	if len(endpoints) == 0 {
		return nil, errors.New("no endpoint available")
	}
	var err error
	var bytes []byte
	for _, endpoint := range endpoints {
		var u *url.URL
		u, err = url.Parse(endpoint)

		if err != nil {
			return bytes, perrs.AddStack(err)
		}

		endpoint = u.String()

		bytes, err = f(endpoint)
		if err != nil {
			continue
		}
		return bytes, nil
	}
	if len(endpoints) > 1 && err != nil {
		err = perrs.Errorf("no endpoint available, the last err was: %s", err)
	}
	return bytes, err
}

func (pc *PDClient) getEndpoints(uri string) (endpoints []string) {
	for _, addr := range pc.addrs {
		endpoint := fmt.Sprintf("%s/%s", pc.GetURL(addr), uri)
		endpoints = append(endpoints, endpoint)
	}

	return
}

// CheckHealth checks the health of PD node
func (pc *PDClient) CheckHealth() error {
	endpoints := pc.getEndpoints(pdPingURI)

	_, err := tryURLs(endpoints, func(endpoint string) ([]byte, error) {
		body, err := pc.httpClient.Get(pc.ctx, endpoint)
		if err != nil {
			return body, err
		}

		return body, nil
	})

	if err != nil {
		return err
	}

	return nil
}

// GetStores queries the stores info from PD server
func (pc *PDClient) GetStores() (*StoresInfo, error) {
	// Return all stores
	query := "?state=0&state=1&state=2"
	endpoints := pc.getEndpoints(pdStoresURI + query)

	storesInfo := StoresInfo{}

	_, err := tryURLs(endpoints, func(endpoint string) ([]byte, error) {
		body, err := pc.httpClient.Get(pc.ctx, endpoint)
		if err != nil {
			return body, err
		}

		return body, json.Unmarshal(body, &storesInfo)
	})
	if err != nil {
		return nil, err
	}

	// Desc sorting the store list, we assume the store with largest ID is the
	// latest one.
	// Not necessary when we implement the workaround pd-3303 in GetCurrentStore()
	// sort.Slice(storesInfo.Stores, func(i int, j int) bool {
	//	 return storesInfo.Stores[i].Store.Id > storesInfo.Stores[j].Store.Id
	// })

	return &storesInfo, nil
}

// GetCurrentStore gets the current store info of a given host
func (pc *PDClient) GetCurrentStore(addr string) (*StoreInfo, error) {
	stores, err := pc.GetStores()
	if err != nil {
		return nil, err
	}

	// Find the store with largest ID
	var latestStore *StoreInfo
	for _, store := range stores.Stores {
		if store.Store.Address == addr {
			// Workaround of pd-3303:
			// If the PD leader has been switched multiple times, the store IDs
			// may be not monitonically assigned. To workaround this, we iterate
			// over the whole store list to see if any of the store's state is
			// not marked as "tombstone", then use that as the result.
			// See: https://github.com/tikv/pd/issues/3303
			//
			// It's logically not necessary to find the store with largest ID
			// number anymore in this process, but we're keeping the behavior
			// as the reasonable approach would still be using the state from
			// latest store, and this is only a workaround.
			if store.Store.State != metapb.StoreState_Tombstone {
				return store, nil
			}

			if latestStore == nil {
				latestStore = store
				continue
			}
			if store.Store.Id > latestStore.Store.Id {
				latestStore = store
			}
		}
	}
	if latestStore != nil {
		return latestStore, nil
	}
	return nil, &NoStoreErr{addr: addr}
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
		pc.l().Debugf("Still waitting for the PD leader to be elected")
		return perrs.New("still waitting for the PD leader to be elected")
	}, *retryOpt); err != nil {
		return fmt.Errorf("error getting PD leader, %v", err)
	}
	return nil
}

// GetLeader queries the leader node of PD cluster
func (pc *PDClient) GetLeader() (*pdpb.Member, error) {
	endpoints := pc.getEndpoints(pdLeaderURI)

	leader := pdpb.Member{}

	_, err := tryURLs(endpoints, func(endpoint string) ([]byte, error) {
		body, err := pc.httpClient.Get(pc.ctx, endpoint)
		if err != nil {
			return body, err
		}

		return body, json.Unmarshal(body, &leader)
	})

	if err != nil {
		return nil, err
	}

	return &leader, nil
}

// GetMembers queries for member list from the PD server
func (pc *PDClient) GetMembers() (*pdpb.GetMembersResponse, error) {
	endpoints := pc.getEndpoints(pdMembersURI)
	members := pdpb.GetMembersResponse{}

	_, err := tryURLs(endpoints, func(endpoint string) ([]byte, error) {
		body, err := pc.httpClient.Get(pc.ctx, endpoint)
		if err != nil {
			return body, err
		}

		return body, json.Unmarshal(body, &members)
	})

	if err != nil {
		return nil, err
	}

	return &members, nil
}

// GetConfig returns all PD configs
func (pc *PDClient) GetConfig() (map[string]interface{}, error) {
	endpoints := pc.getEndpoints(pdConfigURI)

	// We don't use the `github.com/tikv/pd/server/config` directly because
	// there is compatible issue: https://github.com/pingcap/tiup/issues/637
	pdConfig := map[string]interface{}{}

	_, err := tryURLs(endpoints, func(endpoint string) ([]byte, error) {
		body, err := pc.httpClient.Get(pc.ctx, endpoint)
		if err != nil {
			return body, err
		}

		return body, json.Unmarshal(body, &pdConfig)
	})
	if err != nil {
		return nil, err
	}

	return flatten.Flatten(pdConfig, "", flatten.DotStyle)
}

// GetClusterID return cluster ID
func (pc *PDClient) GetClusterID() (uint64, error) {
	endpoints := pc.getEndpoints(pdClusterIDURI)
	var clusterID map[string]interface{}

	_, err := tryURLs(endpoints, func(endpoint string) ([]byte, error) {
		body, err := pc.httpClient.Get(pc.ctx, endpoint)
		if err != nil {
			return body, err
		}
		d := json.NewDecoder(bytes.NewBuffer(body))
		d.UseNumber()
		clusterID = make(map[string]interface{})
		return nil, d.Decode(&clusterID)
	})
	if err != nil {
		return 0, err
	}

	idStr := clusterID["id"].(json.Number).String()
	return strconv.ParseUint(idStr, 10, 64)
}

// GetDashboardAddress get the PD node address which runs dashboard
func (pc *PDClient) GetDashboardAddress() (string, error) {
	cfg, err := pc.GetConfig()
	if err != nil {
		return "", perrs.AddStack(err)
	}

	addr, ok := cfg["pd-server.dashboard-address"].(string)
	if !ok {
		return "", perrs.New("cannot found dashboard address")
	}
	return addr, nil
}

// EvictPDLeader evicts the PD leader
func (pc *PDClient) EvictPDLeader(retryOpt *utils.RetryOption) error {
	// get current members
	members, err := pc.GetMembers()
	if err != nil {
		return err
	}

	if len(members.Members) == 1 {
		pc.l().Warnf("Only 1 member in the PD cluster, skip leader evicting")
		return nil
	}

	// try to evict the leader
	cmd := fmt.Sprintf("%s/resign", pdLeaderURI)
	endpoints := pc.getEndpoints(cmd)

	_, err = tryURLs(endpoints, func(endpoint string) ([]byte, error) {
		body, err := pc.httpClient.Post(pc.ctx, endpoint, nil)
		if err != nil {
			return body, err
		}
		return body, nil
	})

	if err != nil {
		return err
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
		pc.l().Debugf("Still waitting for the PD leader to transfer")
		return perrs.New("still waitting for the PD leader to transfer")
	}, *retryOpt); err != nil {
		return fmt.Errorf("error evicting PD leader, %v", err)
	}
	return nil
}

// pdSchedulerRequest is the request body when evicting store leader
type pdSchedulerRequest struct {
	Name    string `json:"name"`
	StoreID uint64 `json:"store_id"`
}

// EvictStoreLeader evicts the store leaders
// The host parameter should be in format of IP:Port, that matches store's address
func (pc *PDClient) EvictStoreLeader(host string, retryOpt *utils.RetryOption, countLeader func(string) (int, error)) error {
	// get info of current stores
	latestStore, err := pc.GetCurrentStore(host)
	if err != nil {
		if errors.Is(err, ErrNoStore) {
			return nil
		}
		return err
	}

	// XXX: the status address in store will be something like 0.0.0.0:20180
	var leaderCount int
	if leaderCount, err = countLeader(latestStore.Store.Address); err != nil {
		return err
	}
	if leaderCount == 0 {
		// no store leader on the host, just skip
		return nil
	}

	pc.l().Infof("\tEvicting %d leaders from store %s...", leaderCount, latestStore.Store.Address)

	// set scheduler for stores
	scheduler, err := json.Marshal(pdSchedulerRequest{
		Name:    pdEvictLeaderName,
		StoreID: latestStore.Store.Id,
	})
	if err != nil {
		return nil
	}

	endpoints := pc.getEndpoints(pdSchedulersURI)

	_, err = tryURLs(endpoints, func(endpoint string) ([]byte, error) {
		return pc.httpClient.Post(pc.ctx, endpoint, bytes.NewBuffer(scheduler))
	})
	if err != nil {
		return err
	}

	// wait for the transfer to complete
	if retryOpt == nil {
		retryOpt = &utils.RetryOption{
			Delay:   time.Second * 5,
			Timeout: time.Second * 600,
		}
	}
	if err := utils.Retry(func() error {
		currStore, err := pc.GetCurrentStore(host)
		if err != nil {
			if errors.Is(err, ErrNoStore) {
				return nil
			}
			return err
		}

		// check if all leaders are evicted
		if leaderCount, err = countLeader(currStore.Store.Address); err != nil {
			return err
		}
		if leaderCount == 0 {
			return nil
		}
		pc.l().Infof(
			"\t  Still waitting for %d store leaders to transfer...",
			leaderCount,
		)

		// return error by default, to make the retry work
		return perrs.New("still waiting for the store leaders to transfer")
	}, *retryOpt); err != nil {
		return fmt.Errorf("error evicting store leader from %s, %v", host, err)
	}
	return nil
}

// RemoveStoreEvict removes a store leader evict scheduler, which allows following
// leaders to be transffered to it again.
func (pc *PDClient) RemoveStoreEvict(host string) error {
	// get info of current stores
	latestStore, err := pc.GetCurrentStore(host)
	if err != nil {
		return err
	}

	// remove scheduler for the store
	cmd := fmt.Sprintf(
		"%s/%s",
		pdSchedulersURI,
		fmt.Sprintf("%s-%d", pdEvictLeaderName, latestStore.Store.Id),
	)
	endpoints := pc.getEndpoints(cmd)

	logger := pc.l()
	_, err = tryURLs(endpoints, func(endpoint string) ([]byte, error) {
		body, statusCode, err := pc.httpClient.Delete(pc.ctx, endpoint, nil)
		if err != nil {
			if statusCode == http.StatusNotFound || bytes.Contains(body, []byte("scheduler not found")) {
				logger.Debugf("Store leader evicting scheduler does not exist, ignore.")
				return body, nil
			}
			return body, err
		}
		logger.Debugf("Delete leader evicting scheduler of store %d success", latestStore.Store.Id)
		return body, nil
	})
	if err != nil {
		return err
	}

	logger.Debugf("Removed store leader evicting scheduler from %s.", latestStore.Store.Address)
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
		return perrs.New("at least 1 PD node must be online, can not delete")
	}

	// try to delete the node
	cmd := fmt.Sprintf("%s/name/%s", pdMembersURI, name)
	endpoints := pc.getEndpoints(cmd)

	logger := pc.l()
	_, err = tryURLs(endpoints, func(endpoint string) ([]byte, error) {
		body, statusCode, err := pc.httpClient.Delete(pc.ctx, endpoint, nil)
		if err != nil {
			if statusCode == http.StatusNotFound || bytes.Contains(body, []byte("not found, pd")) {
				logger.Debugf("PD node does not exist, ignore: %s", body)
				return body, nil
			}
			return body, err
		}
		logger.Debugf("Delete PD %s from the cluster success", name)
		return body, nil
	})
	if err != nil {
		return err
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
				return perrs.New("still waitting for the PD node to be deleted")
			}
		}

		return nil
	}, *retryOpt); err != nil {
		return fmt.Errorf("error deleting PD node, %v", err)
	}
	return nil
}

func (pc *PDClient) isSameState(host string, state metapb.StoreState) (bool, error) {
	// get info of current stores
	storeInfo, err := pc.GetCurrentStore(host)
	if err != nil {
		return false, err
	}

	if storeInfo.Store.State == state {
		return true, nil
	}

	return false, nil
}

// IsTombStone check if the node is Tombstone.
// The host parameter should be in format of IP:Port, that matches store's address
func (pc *PDClient) IsTombStone(host string) (bool, error) {
	return pc.isSameState(host, metapb.StoreState_Tombstone)
}

// IsUp check if the node is Up state.
// The host parameter should be in format of IP:Port, that matches store's address
func (pc *PDClient) IsUp(host string) (bool, error) {
	return pc.isSameState(host, metapb.StoreState_Up)
}

// DelStore deletes stores from a (TiKV) host
// The host parameter should be in format of IP:Port, that matches store's address
func (pc *PDClient) DelStore(host string, retryOpt *utils.RetryOption) error {
	// get info of current stores
	storeInfo, err := pc.GetCurrentStore(host)
	if err != nil {
		if errors.Is(err, ErrNoStore) {
			return nil
		}
		return err
	}

	// get store ID of host
	storeID := storeInfo.Store.Id

	cmd := fmt.Sprintf("%s/%d", pdStoreURI, storeID)
	endpoints := pc.getEndpoints(cmd)

	logger := pc.l()
	_, err = tryURLs(endpoints, func(endpoint string) ([]byte, error) {
		body, statusCode, err := pc.httpClient.Delete(pc.ctx, endpoint, nil)
		if err != nil {
			if statusCode == http.StatusNotFound || bytes.Contains(body, []byte("not found")) {
				logger.Debugf("store %d %s does not exist, ignore: %s", storeID, host, body)
				return body, nil
			}
			return body, err
		}
		logger.Debugf("Delete store %d %s from the cluster success", storeID, host)
		return body, nil
	})
	if err != nil {
		return err
	}

	// wait for the deletion to complete
	if retryOpt == nil {
		retryOpt = &utils.RetryOption{
			Delay:   time.Second * 2,
			Timeout: time.Second * 60,
		}
	}
	if err := utils.Retry(func() error {
		currStore, err := pc.GetCurrentStore(host)
		if err != nil {
			// the store does not exist anymore, just ignore and skip
			if errors.Is(err, ErrNoStore) {
				return nil
			}
			return err
		}

		if currStore.Store.Id == storeID {
			// deleting a store may take long time to transfer data, so we
			// return success once it get to "Offline" status and not waiting
			// for the whole process to complete.
			// When finished, the store's state will be "Tombstone".
			if currStore.Store.State != metapb.StoreState_Up {
				return nil
			}
			return perrs.New("still waiting for the store to be deleted")
		}

		return nil
	}, *retryOpt); err != nil {
		return fmt.Errorf("error deleting store, %v", err)
	}
	return nil
}

func (pc *PDClient) updateConfig(url string, body io.Reader) error {
	endpoints := pc.getEndpoints(url)
	_, err := tryURLs(endpoints, func(endpoint string) ([]byte, error) {
		return pc.httpClient.Post(pc.ctx, endpoint, body)
	})
	return err
}

// UpdateReplicateConfig updates the PD replication config
func (pc *PDClient) UpdateReplicateConfig(body io.Reader) error {
	return pc.updateConfig(pdConfigReplicate, body)
}

// GetReplicateConfig gets the PD replication config
func (pc *PDClient) GetReplicateConfig() ([]byte, error) {
	endpoints := pc.getEndpoints(pdConfigReplicate)
	return tryURLs(endpoints, func(endpoint string) ([]byte, error) {
		return pc.httpClient.Get(pc.ctx, endpoint)
	})
}

// GetLocationLabels gets the replication.location-labels config from pd server
func (pc *PDClient) GetLocationLabels() ([]string, bool, error) {
	config, err := pc.GetReplicateConfig()
	if err != nil {
		return nil, false, err
	}

	rc := PDReplicationConfig{}
	if err := json.Unmarshal(config, &rc); err != nil {
		return nil, false, perrs.Annotatef(err, "unmarshal replication config: %s", string(config))
	}

	return rc.LocationLabels, rc.EnablePlacementRules, nil
}

// GetTiKVLabels implements TiKVLabelProvider
func (pc *PDClient) GetTiKVLabels() (map[string]map[string]string, []map[string]LabelInfo, error) {
	r, err := pc.GetStores()
	if err != nil {
		return nil, nil, err
	}

	var storeInfo []map[string]LabelInfo

	locationLabels := map[string]map[string]string{}

	for _, s := range r.Stores {
		if s.Store.State == metapb.StoreState_Up {
			lbs := s.Store.GetLabels()
			labelsMap := map[string]string{}

			var labelsArr []string

			for _, lb := range lbs {
				// Skip tiflash
				if lb.GetKey() != "tiflash" {
					labelsArr = append(labelsArr, fmt.Sprintf("%s: %s", lb.GetKey(), lb.GetValue()))
					labelsMap[lb.GetKey()] = lb.GetValue()
				}
			}

			locationLabels[s.Store.GetAddress()] = labelsMap

			label := fmt.Sprintf("%s%s%s", "{", strings.Join(labelsArr, ","), "}")
			storeInfo = append(storeInfo, map[string]LabelInfo{
				strings.Split(s.Store.GetAddress(), ":")[0]: {
					Machine:   strings.Split(s.Store.GetAddress(), ":")[0],
					Port:      strings.Split(s.Store.GetAddress(), ":")[1],
					Store:     s.Store.GetId(),
					Status:    s.Store.State.String(),
					Leaders:   s.Status.LeaderCount,
					Regions:   s.Status.RegionCount,
					Capacity:  s.Status.Capacity.MarshalString(),
					Available: s.Status.Available.MarshalString(),
					Labels:    label,
				},
			})
		}
	}
	return locationLabels, storeInfo, nil
}

// UpdateScheduleConfig updates the PD schedule config
func (pc *PDClient) UpdateScheduleConfig(body io.Reader) error {
	return pc.updateConfig(pdConfigSchedule, body)
}

// CheckRegion queries for the region with specific status
func (pc *PDClient) CheckRegion(state string) (*RegionsInfo, error) {
	uri := pdRegionsCheckURI + "/" + state
	endpoints := pc.getEndpoints(uri)
	regionsInfo := RegionsInfo{}

	_, err := tryURLs(endpoints, func(endpoint string) ([]byte, error) {
		body, err := pc.httpClient.Get(pc.ctx, endpoint)
		if err != nil {
			return body, err
		}

		return body, json.Unmarshal(body, &regionsInfo)
	})
	return &regionsInfo, err
}

// SetReplicationConfig sets a config key value of PD replication, it has the
// same effect as `pd-ctl config set key value`
func (pc *PDClient) SetReplicationConfig(key string, value int) error {
	// Only support for pd version >= v4.0.0
	if pc.version == "" || semver.Compare(pc.version, "v4.0.0") < 0 {
		return nil
	}

	data := map[string]interface{}{"set": map[string]interface{}{key: value}}
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}
	pc.l().Debugf("setting replication config: %s=%d", key, value)
	return pc.updateConfig(pdReplicationModeURI, bytes.NewBuffer(body))
}

// SetAllStoreLimits sets store for all stores and types, it has the same effect
// as `pd-ctl store limit all value`
func (pc *PDClient) SetAllStoreLimits(value int) error {
	// Only support for pd version >= v4.0.0
	if pc.version == "" || semver.Compare(pc.version, "v4.0.0") < 0 {
		return nil
	}

	data := map[string]interface{}{"rate": value}
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}
	pc.l().Debugf("setting store limit: %d", value)
	return pc.updateConfig(pdStoresLimitURI, bytes.NewBuffer(body))
}
