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

package telemetry

import (
	"crypto/md5"
	"fmt"
	"reflect"

	"gopkg.in/yaml.v2"
)

// ScrubStrategy for scrub sensible value.
type ScrubStrategy int

// HashReport return the hash value of val.
func HashReport(val string) string {
	s := fmt.Sprintf("%x", md5.Sum([]byte(val)))
	return s
}

// ScrubYaml scrub the value.
// for string type, replace as "_", unless the field name is in the hashFieldNames.
// for any other type set as the zero value of the according type.
func ScrubYaml(data []byte, hashFieldNames map[string]struct{}) (scrubed []byte, err error) {
	mp := make(map[interface{}]interface{})
	err = yaml.Unmarshal(data, mp)
	if err != nil {
		return nil, err
	}

	smp := scrupMap(mp, hashFieldNames, false).(map[string]interface{})
	scrubed, err = yaml.Marshal(smp)
	return
}

func scrupMap(val interface{}, hashFieldNames map[string]struct{}, hash bool) interface{} {
	if val == nil {
		return nil
	}

	m, ok := val.(map[interface{}]interface{})
	if ok {
		ret := make(map[string]interface{})
		for k, v := range m {
			kk, ok := k.(string)
			if !ok {
				return val
			}
			_, hash = hashFieldNames[kk]
			ret[kk] = scrupMap(v, hashFieldNames, hash)
		}
		return ret
	}

	rv := reflect.ValueOf(val)
	if rv.Kind() == reflect.Slice {
		var ret []interface{}
		for i := 0; i < rv.Len(); i++ {
			ret = append(ret, scrupMap(rv.Index(i).Interface(), hashFieldNames, false))
		}
		return ret
	}

	if rv.Kind() == reflect.String {
		if hash {
			return HashReport(rv.String())
		}
		return "_"
	}

	return reflect.Zero(rv.Type()).Interface()
}
