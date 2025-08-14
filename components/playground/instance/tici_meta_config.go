// Copyright 2025 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE_2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package instance

import (
	"fmt"
)

func warpAddr(addr string) string {
	return fmt.Sprintf("http://%s", addr)
}

func (inst *TiCIInstance) getMetaConfig() map[string]any {
	config := make(map[string]any)
	tidbServers := make([]string, 0, len(inst.dbs))
	for _, db := range inst.dbs {
		tidbServers = append(tidbServers, db.DSN())
	}
	config["tidb_servers"] = tidbServers
	config["pd_addr"] = warpAddr(inst.pds[0].Addr())
	config["cert_path"] = ""
	config["addr"] = warpAddr(inst.Addr())
	config["advertise_addr"] = warpAddr(inst.Addr())
	config["status_addr"] = warpAddr(inst.StatusAddr())
	config["advertise_status_addr"] = warpAddr(inst.StatusAddr())

	// reader pool config
	config["reader_pool.ttl_seconds"] = 9
	config["reader_pool.cleanup_interval_seconds"] = 1
	config["reader_pool.scheduling_strategy"] = "round_robin"

	// S3 config
	// TODO: make it configurable
	endpoint, ak, sk, bucket, prefix := GetDefaultTiCIMetaS3Config()
	config["s3.endpoint"] = endpoint
	config["s3.region"] = "us_east_1"
	config["s3.access_key"] = ak
	config["s3.secret_key"] = sk
	config["s3.use_path_style"] = false
	config["s3.bucket"] = bucket
	config["s3.prefix"] = prefix

	// shard config
	config["shard.compaction_fragments"] = 10
	config["shard.compaction_datafiles"] = 300
	config["shard.max_size"] = "1024MB"
	config["shard.split_threshold"] = 0.75

	return config
}

// GetDefaultTiCIMetaS3Config returns the default S3 configuration for TiCI Meta Service
func GetDefaultTiCIMetaS3Config() (string, string, string, string, string) {
	return "http://localhost:9000", "minioadmin", "minioadmin", "logbucket", "storage_test"
}
