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

package instance

func (inst *TiKVInstance) getConfig() map[string]any {
	config := make(map[string]any)
	config["rocksdb.max-open-files"] = 256
	config["raftdb.max-open-files"] = 256
	config["storage.reserve-space"] = 0
	config["storage.reserve-raft-space"] = 0

	if inst.isCSEMode {
		config["storage.api-version"] = 2
		config["storage.enable-ttl"] = true
		config["dfs.prefix"] = "tikv"
		config["dfs.s3-endpoint"] = inst.cseOpts.S3Endpoint
		config["dfs.s3-key-id"] = inst.cseOpts.AccessKey
		config["dfs.s3-secret-key"] = inst.cseOpts.SecretKey
		config["dfs.s3-bucket"] = inst.cseOpts.Bucket
		config["dfs.s3-region"] = "local"
	}

	return config
}
