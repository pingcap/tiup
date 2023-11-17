// Copyright 2021 PingCAP, Inc.
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

// Some structs are manually copied from other packages like PD, to avoid
// direct depends on them, which will make our dependency tree complicated
// and hard to tidy and upgrade.

package api

import (
	"time"

	"github.com/pingcap/kvproto/pkg/metapb"
	"github.com/pingcap/kvproto/pkg/pdpb"
	"github.com/pingcap/tiup/pkg/cluster/api/typeutil"
)

// PDReplicationConfig is the replication type configuration ReplicationConfig from PD.
type PDReplicationConfig struct {
	// MaxReplicas is the number of replicas for each region.
	MaxReplicas uint64 `toml:"max-replicas" json:"max-replicas"`

	// The label keys specified the location of a store.
	// The placement priorities is implied by the order of label keys.
	// For example, ["zone", "rack"] means that we should place replicas to
	// different zones first, then to different racks if we don't have enough zones.
	LocationLabels typeutil.StringSlice `toml:"location-labels" json:"location-labels"`
	// StrictlyMatchLabel strictly checks if the label of TiKV is matched with LocationLabels.
	StrictlyMatchLabel bool `toml:"strictly-match-label" json:"strictly-match-label,string"`

	// When PlacementRules feature is enabled. MaxReplicas, LocationLabels and IsolationLabels are not used any more.
	EnablePlacementRules bool `toml:"enable-placement-rules" json:"enable-placement-rules,string"`

	// IsolationLevel is used to isolate replicas explicitly and forcibly if it's not empty.
	// Its value must be empty or one of LocationLabels.
	// Example:
	// location-labels = ["zone", "rack", "host"]
	// isolation-level = "zone"
	// With configuration like above, PD ensure that all replicas be placed in different zones.
	// Even if a zone is down, PD will not try to make up replicas in other zone
	// because other zones already have replicas on it.
	IsolationLevel string `toml:"isolation-level" json:"isolation-level"`
}

// MetaStore contains meta information about a store.
type MetaStore struct {
	*metapb.Store
	StateName string `json:"state_name"`
}

// StoreStatus contains status about a store.
type StoreStatus struct {
	Capacity           typeutil.ByteSize  `json:"capacity"`
	Available          typeutil.ByteSize  `json:"available"`
	UsedSize           typeutil.ByteSize  `json:"used_size"`
	LeaderCount        int                `json:"leader_count"`
	LeaderWeight       float64            `json:"leader_weight"`
	LeaderScore        float64            `json:"leader_score"`
	LeaderSize         int64              `json:"leader_size"`
	RegionCount        int                `json:"region_count"`
	RegionWeight       float64            `json:"region_weight"`
	RegionScore        float64            `json:"region_score"`
	RegionSize         int64              `json:"region_size"`
	SlowScore          uint64             `json:"slow_score,omitempty"` // added omitempty
	SendingSnapCount   uint32             `json:"sending_snap_count,omitempty"`
	ReceivingSnapCount uint32             `json:"receiving_snap_count,omitempty"`
	ApplyingSnapCount  uint32             `json:"applying_snap_count,omitempty"`
	IsBusy             bool               `json:"is_busy,omitempty"`
	StartTS            *time.Time         `json:"start_ts,omitempty"`
	LastHeartbeatTS    *time.Time         `json:"last_heartbeat_ts,omitempty"`
	Uptime             *typeutil.Duration `json:"uptime,omitempty"`
}

// StoreInfo contains information about a store.
type StoreInfo struct {
	Store  *MetaStore   `json:"store"`
	Status *StoreStatus `json:"status"`
}

// StoresInfo records stores' info.
type StoresInfo struct {
	Count  int          `json:"count"`
	Stores []*StoreInfo `json:"stores"`
}

// ReplicationStatus represents the replication mode status of the region.
type ReplicationStatus struct {
	State   string `json:"state"`
	StateID uint64 `json:"state_id"`
}

// MetaPeer is api compatible with *metapb.Peer.
type MetaPeer struct {
	*metapb.Peer
	// RoleName is `Role.String()`.
	// Since Role is serialized as int by json by default,
	// introducing it will make the output of pd-ctl easier to identify Role.
	RoleName string `json:"role_name"`
	// IsLearner is `Role == "Learner"`.
	// Since IsLearner was changed to Role in kvproto in 5.0, this field was introduced to ensure api compatibility.
	IsLearner bool `json:"is_learner,omitempty"`
}

// PDPeerStats is api compatible with *pdpb.PeerStats.
// NOTE: This type is exported by HTTP API. Please pay more attention when modifying it.
type PDPeerStats struct {
	*pdpb.PeerStats
	Peer MetaPeer `json:"peer"`
}

// RegionInfo records detail region info for api usage.
type RegionInfo struct {
	ID          uint64              `json:"id"`
	StartKey    string              `json:"start_key"`
	EndKey      string              `json:"end_key"`
	RegionEpoch *metapb.RegionEpoch `json:"epoch,omitempty"`
	Peers       []MetaPeer          `json:"peers,omitempty"`

	Leader          MetaPeer      `json:"leader,omitempty"`
	DownPeers       []PDPeerStats `json:"down_peers,omitempty"`
	PendingPeers    []MetaPeer    `json:"pending_peers,omitempty"`
	WrittenBytes    uint64        `json:"written_bytes"`
	ReadBytes       uint64        `json:"read_bytes"`
	WrittenKeys     uint64        `json:"written_keys"`
	ReadKeys        uint64        `json:"read_keys"`
	ApproximateSize int64         `json:"approximate_size"`
	ApproximateKeys int64         `json:"approximate_keys"`
	Buckets         []string      `json:"buckets,omitempty"`

	ReplicationStatus *ReplicationStatus `json:"replication_status,omitempty"`
}

// RegionsInfo contains some regions with the detailed region info.
type RegionsInfo struct {
	Count   int           `json:"count"`
	Regions []*RegionInfo `json:"regions"`
}
