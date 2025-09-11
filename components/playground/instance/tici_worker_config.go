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

func (inst *TiCIInstance) getWorkerConfig() map[string]any {
	config := make(map[string]any)

	// S3 config
	// TODO: make it configurable
	config["s3.endpoint"] = "http://localhost:9000"
	config["s3.region"] = "us_east_1"
	config["s3.access_key"] = "minioadmin"
	config["s3.secret_key"] = "minioadmin"
	config["s3.use_path_style"] = true

	// fragment writer config
	config["frag_writer.local_data_path"] = "fragments"
	config["frag_writer.index_num_threads"] = 4
	config["frag_writer.index_mem_budget"] = "255MB"
	config["frag_writer.index_flush_interval"] = "5s"
	config["frag_writer.index_flush_size_limit"] = "5MB"

	config["heartbeat_interval"] = "3s"

	return config
}
