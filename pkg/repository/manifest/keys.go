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

package manifest

// KeyInfo is the manifest structure of a single key
type KeyInfo struct {
	Algorithms []string          `json:"keyid_hash_algorithms"`
	Type       string            `json:"keytype"`
	Value      map[string]string `json:"keyval"`
	Scheme     string            `json:"scheme"`
}
