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

package scripts

import (
	"path/filepath"

	"github.com/pingcap/tiup/embed"
)

// GetScript returns a raw config file from embed templates
func GetScript(filename string) ([]byte, error) {
	fp := filepath.Join("templates", "scripts", filename)
	return embed.ReadFile(fp)
}
