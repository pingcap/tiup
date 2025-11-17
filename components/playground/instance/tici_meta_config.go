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

func (inst *TiCIInstance) getMetaConfig() map[string]any {
	config := make(map[string]any)
	tidbServers := make([]string, 0, len(inst.dbs))
	for _, db := range inst.dbs {
		tidbServers = append(tidbServers, db.DSN())
	}
	config["tidb_server.dsns"] = tidbServers
	config["s3.endpoint"] = inst.shOpt.S3.Endpoint
	config["s3.access_key"] = inst.shOpt.S3.AccessKey
	config["s3.secret_key"] = inst.shOpt.S3.SecretKey
	config["s3.bucket"] = inst.shOpt.S3.Bucket
	config["s3.prefix"] = inst.shOpt.S3.Prefix
	return config
}
