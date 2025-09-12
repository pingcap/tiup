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

import "github.com/pingcap/tiup/pkg/utils"

func (inst *TiFlashInstance) getTiCIReaderConfig() map[string]any {
	host := AdvertiseHost(inst.Host)
	config := make(map[string]any)

	config["tici.enable"] = true
	// S3 config
	config["tici.s3.endpoint"] = "http://localhost:9000"
	config["tici.s3.region"] = "us-east-1"
	config["tici.s3.access_key"] = "minioadmin"
	config["tici.s3.secret_key"] = "minioadmin"
	config["tici.s3.use_path_style"] = true

	// Reader node config
	config["tici.reader_node.addr"] = utils.JoinHostPort(host, inst.ticReaderPort)
	config["tici.reader_node.heartbeat_interval"] = "3s"
	config["tici.reader_node.max_heartbeat_retries"] = 3

	// Fragment reader config
	config["tici.frag_reader.local_data_path"] = "frag_local_data"
	config["tici.frag_reader.doc_store_cache_size"] = "16MB"

	// TiFlash config
	config["flash.service_addr"] = utils.JoinHostPort(host, inst.servicePort)

	config["raft.pd_addr"] = inst.pds[0].Addr()

	return config
}
