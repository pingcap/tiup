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
	tiupmeta "github.com/pingcap/tiup/pkg/environment"
)

var _env *tiupmeta.Environment

// SetTiupEnv the gloable env used.
func SetTiupEnv(env *tiupmeta.Environment) {
	_env = env
}

// TiupEnv Get the gloable env used.
func TiupEnv() *tiupmeta.Environment {
	return _env
}
