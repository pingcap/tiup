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

package meta

import (
	"fmt"
)

// DirPaths stores the paths needed for component to put files
type DirPaths struct {
	Deploy string
	Data   []string
	Log    string
	Cache  string
}

// String implements the fmt.Stringer interface
func (p DirPaths) String() string {
	return fmt.Sprintf(
		"deploy_dir=%s, data_dir=%v, log_dir=%s, cache_dir=%s",
		p.Deploy,
		p.Data,
		p.Log,
		p.Cache,
	)
}
