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
	"reflect"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/pingcap/errors"
)

// strKeyMap tries to convert `map[interface{}]interface{}` to `map[string]interface{}`
func strKeyMap(val interface{}) interface{} {
	m, ok := val.(map[interface{}]interface{})
	if ok {
		ret := map[string]interface{}{}
		for k, v := range m {
			kk, ok := k.(string)
			if !ok {
				return val
			}
			ret[kk] = strKeyMap(v)
		}
		return ret
	}

	rv := reflect.ValueOf(val)
	if rv.Kind() == reflect.Slice {
		var ret []interface{}
		for i := 0; i < rv.Len(); i++ {
			ret = append(ret, strKeyMap(rv.Index(i).Interface()))
		}
		return ret
	}

	return val
}

func flattenKey(key string, val interface{}) (string, interface{}) {
	parts := strings.SplitN(key, ".", 2)
	if len(parts) == 1 {
		return key, strKeyMap(val)
	}
	subKey, subVal := flattenKey(parts[1], val)
	return parts[0], map[string]interface{}{
		subKey: strKeyMap(subVal),
	}
}

func patch(origin map[string]interface{}, key string, val interface{}) {
	origVal, found := origin[key]
	if !found {
		origin[key] = strKeyMap(val)
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
		origin[key] = strKeyMap(val)
	}
}

func flattenMap(ms map[string]interface{}) (map[string]interface{}, error) {
	result := map[string]interface{}{}
	for k, v := range ms {
		key, val := flattenKey(k, v)
		patch(result, key, val)
	}
	return result, nil
}

func merge(global, overwrite map[string]interface{}) (map[string]interface{}, error) {
	lhs, err := flattenMap(global)
	if err != nil {
		return nil, err
	}
	rhs, err := flattenMap(overwrite)
	if err != nil {
		return nil, err
	}
	for k, v := range rhs {
		patch(lhs, k, v)
	}
	return lhs, nil
}

func merge2Toml(comp string, global, overwrite map[string]interface{}) ([]byte, error) {
	lhs, err := merge(global, overwrite)
	if err != nil {
		return nil, errors.AddStack(err)
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
