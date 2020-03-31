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
	"bytes"
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/pingcap/errors"
	"gopkg.in/yaml.v2"
)

func flattenKey(key string, val interface{}) (string, interface{}) {
	parts := strings.SplitN(key, ".", 2)
	if len(parts) == 1 {
		return key, val
	}
	subKey, subVal := flattenKey(parts[1], val)
	return parts[0], map[string]interface{}{
		subKey: subVal,
	}
}

func patch(origin map[string]interface{}, key string, val interface{}) {
	origVal, found := origin[key]
	if !found {
		origin[key] = val
		return
	}
	origMap, lhsOk := origVal.(map[string]interface{})
	valMap, rhsOk := val.(map[string]interface{})
	if lhsOk && rhsOk {
		for k, v := range valMap {
			patch(origMap, k, v)
		}
	} else {
		// overwrite
		origin[key] = val
	}
}

func toMap(ms yaml.MapSlice) (map[string]interface{}, error) {
	result := map[string]interface{}{}
	for _, item := range ms {
		s, ok := item.Key.(string)
		if !ok {
			return nil, errors.Errorf("unrecognized key type: %T(%+v)", item.Key, item.Key)
		}
		key, val := flattenKey(s, item.Value)
		patch(result, key, val)
	}
	return result, nil
}

func merge2Toml(comp string, global, overwrite yaml.MapSlice) ([]byte, error) {
	lhs, err := toMap(global)
	if err != nil {
		return nil, err
	}
	rhs, err := toMap(overwrite)
	if err != nil {
		return nil, err
	}
	for k, v := range rhs {
		patch(lhs, k, v)
	}

	buf := bytes.NewBufferString(fmt.Sprintf(`# WARNING: This file was auto-generated. Do not edit! All your edit might be overwritten!
# You can use 'tiup cluster edit-config' and 'tiup cluster reload' to update the configuration
# All configuration items you want to change can be added to:
# server_configs:
#   %s:
#     aa.b1.c3: value
#     aa.b2.c4: value
`, comp))
	enc := toml.NewEncoder(buf)
	enc.Indent = ""
	err = enc.Encode(lhs)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
