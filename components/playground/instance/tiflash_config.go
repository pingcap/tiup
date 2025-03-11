// Copyright 2023 PingCAP, Inc.
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

package instance

import "path/filepath"

func (inst *TiFlashInstance) getProxyConfig() map[string]any {
	config := make(map[string]any)
	config["rocksdb.max-open-files"] = 256
	config["raftdb.max-open-files"] = 256
	config["storage.reserve-space"] = 0
	config["storage.reserve-raft-space"] = 0

	if inst.Role == TiFlashRoleDisaggWrite {
		if inst.mode == "tidb-cse" {
			config["storage.api-version"] = 2
			config["storage.enable-ttl"] = true
			config["dfs.prefix"] = "tikv"
			config["dfs.s3-endpoint"] = inst.cseOpts.S3Endpoint
			config["dfs.s3-key-id"] = inst.cseOpts.AccessKey
			config["dfs.s3-secret-key"] = inst.cseOpts.SecretKey
			config["dfs.s3-bucket"] = inst.cseOpts.Bucket
			config["dfs.s3-region"] = "local"
		}
	}

	return config
}

func (inst *TiFlashInstance) getConfig() map[string]any {
	config := make(map[string]any)

	config["flash.proxy.config"] = filepath.Join(inst.Dir, "tiflash_proxy.toml")
	config["logger.level"] = "debug"

	if inst.Role == TiFlashRoleDisaggWrite {
		config["storage.s3.endpoint"] = inst.cseOpts.S3Endpoint
		config["storage.s3.bucket"] = inst.cseOpts.Bucket
		config["storage.s3.root"] = "/tiflash-cse/"
		config["storage.s3.access_key_id"] = inst.cseOpts.AccessKey
		config["storage.s3.secret_access_key"] = inst.cseOpts.SecretKey
		config["storage.main.dir"] = []string{filepath.Join(inst.Dir, "main_data")}
		config["flash.disaggregated_mode"] = "tiflash_write"
		if inst.mode == "tidb-cse" {
			config["enable_safe_point_v2"] = true
			config["storage.api_version"] = 2
		}
	} else if inst.Role == TiFlashRoleDisaggCompute {
		config["storage.s3.endpoint"] = inst.cseOpts.S3Endpoint
		config["storage.s3.bucket"] = inst.cseOpts.Bucket
		config["storage.s3.root"] = "/tiflash-cse/"
		config["storage.s3.access_key_id"] = inst.cseOpts.AccessKey
		config["storage.s3.secret_access_key"] = inst.cseOpts.SecretKey
		config["storage.remote.cache.dir"] = filepath.Join(inst.Dir, "remote_cache")
		config["storage.remote.cache.capacity"] = 50000000000 // 50GB
		config["storage.main.dir"] = []string{filepath.Join(inst.Dir, "main_data")}
		config["flash.disaggregated_mode"] = "tiflash_compute"
		if inst.mode == "tidb-cse" {
			config["enable_safe_point_v2"] = true
		}
	}

	return config
}
