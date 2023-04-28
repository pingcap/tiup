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
	return config
}

func (inst *TiFlashInstance) getConfig() map[string]any {
	config := make(map[string]any)

	config["flash.proxy.config"] = filepath.Join(inst.Dir, "tiflash_proxy.toml")

	if inst.Role == TiFlashRoleDisaggWrite {
		config["storage.s3.endpoint"] = inst.DisaggOpts.S3Endpoint
		config["storage.s3.bucket"] = inst.DisaggOpts.Bucket
		config["storage.s3.root"] = "/"
		config["storage.s3.access_key_id"] = inst.DisaggOpts.AccessKey
		config["storage.s3.secret_access_key"] = inst.DisaggOpts.SecretKey
		config["flash.disaggregated_mode"] = "tiflash_write"
		config["flash.use_autoscaler"] = false
	} else if inst.Role == TiFlashRoleDisaggCompute {
		config["storage.s3.endpoint"] = inst.DisaggOpts.S3Endpoint
		config["storage.s3.bucket"] = inst.DisaggOpts.Bucket
		config["storage.s3.root"] = "/"
		config["storage.s3.access_key_id"] = inst.DisaggOpts.AccessKey
		config["storage.s3.secret_access_key"] = inst.DisaggOpts.SecretKey
		config["storage.remote.cache.dir"] = filepath.Join(inst.Dir, "remote_cache")
		config["storage.remote.cache.capacity"] = 1000000000 // 1GB
		config["flash.disaggregated_mode"] = "tiflash_compute"
		config["flash.use_autoscaler"] = false
	}

	return config
}
