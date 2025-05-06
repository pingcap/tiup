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
		if inst.shOpt.Mode == "tidb-cse" {
			config["storage.api-version"] = 2
			config["storage.enable-ttl"] = true
			config["dfs.prefix"] = "tikv"
			config["dfs.s3-endpoint"] = inst.shOpt.CSE.S3Endpoint
			config["dfs.s3-key-id"] = inst.shOpt.CSE.AccessKey
			config["dfs.s3-secret-key"] = inst.shOpt.CSE.SecretKey
			config["dfs.s3-bucket"] = inst.shOpt.CSE.Bucket
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
		config["storage.s3.endpoint"] = inst.shOpt.CSE.S3Endpoint
		config["storage.s3.bucket"] = inst.shOpt.CSE.Bucket
		config["storage.s3.root"] = "/tiflash-cse/"
		config["storage.s3.access_key_id"] = inst.shOpt.CSE.AccessKey
		config["storage.s3.secret_access_key"] = inst.shOpt.CSE.SecretKey
		config["storage.main.dir"] = []string{filepath.Join(inst.Dir, "main_data")}
		config["flash.disaggregated_mode"] = "tiflash_write"
		if inst.shOpt.Mode == "tidb-cse" {
			config["enable_safe_point_v2"] = true
			config["storage.api_version"] = 2
		}
	} else if inst.Role == TiFlashRoleDisaggCompute {
		config["storage.s3.endpoint"] = inst.shOpt.CSE.S3Endpoint
		config["storage.s3.bucket"] = inst.shOpt.CSE.Bucket
		config["storage.s3.root"] = "/tiflash-cse/"
		config["storage.s3.access_key_id"] = inst.shOpt.CSE.AccessKey
		config["storage.s3.secret_access_key"] = inst.shOpt.CSE.SecretKey
		config["storage.remote.cache.dir"] = filepath.Join(inst.Dir, "remote_cache")
		config["storage.remote.cache.capacity"] = uint64(50000000000) // 50GB
		config["storage.main.dir"] = []string{filepath.Join(inst.Dir, "main_data")}
		config["flash.disaggregated_mode"] = "tiflash_compute"
		if inst.shOpt.Mode == "tidb-cse" {
			config["enable_safe_point_v2"] = true
		}
	}

	if inst.shOpt.HighPerf {
		config["logger.level"] = "info"
		if inst.Role == TiFlashRoleDisaggWrite {
			config["profiles.default.cpu_thread_count_scale"] = 5.0
		} else if inst.Role == TiFlashRoleDisaggCompute {
			config["profiles.default.task_scheduler_thread_soft_limit"] = 0
			config["profiles.default.task_scheduler_thread_hard_limit"] = 0
		}
	}

	return config
}
