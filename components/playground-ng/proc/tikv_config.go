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

package proc

import "path/filepath"

func (inst *TiKVInstance) getConfig() map[string]any {
	config := make(map[string]any)
	config["rocksdb.max-open-files"] = 256
	config["raftdb.max-open-files"] = 256
	config["storage.reserve-space"] = 0
	config["storage.reserve-raft-space"] = 0

	switch inst.ShOpt.Mode {
	case ModeCSE:
		config["storage.api-version"] = 2
		config["storage.enable-ttl"] = true
		applyS3DFSConfig(config, inst.ShOpt.CSE, "tikv")
		config["kvengine.build-columnar"] = true
	case ModeNextGen:
		config["storage.api-version"] = 2
		config["storage.enable-ttl"] = true
		applyS3DFSConfig(config, inst.ShOpt.CSE, "tikv")
		config["rfengine.wal-sync-dir"] = filepath.Join(inst.Dir, "raft-wal")
		config["rfengine.lightweight-backup"] = true
		config["rfengine.target-file-size"] = "512MB"
		config["rfengine.wal-chunk-target-file-size"] = "128MB"
	}

	return config
}
