// Copyright 2024 PingCAP, Inc.
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

func (inst *PDInstance) getConfig() map[string]any {
	config := make(map[string]any)
	config["schedule.patrol-region-interval"] = "100ms"

	if inst.isCSEMode {
		config["keyspace.pre-alloc"] = []string{"mykeyspace"}
		config["replication.enable-placement-rules"] = true
		config["replication.max-replica"] = 1
		config["schedule.merge-schedule-limit"] = 0
		config["schedule.low-space-ration"] = 1.0
		config["schedule.replica-schedule-limit"] = 500
	}

	return config
}
