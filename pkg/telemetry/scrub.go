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

	"github.com/pingcap/tiup/pkg/set"
	"gopkg.in/yaml.v2"
)

// ScrubStrategy for scrub sensible value.
type ScrubStrategy int

// hashReport return the hash value of val.
func hashReport(val string) string {
	s := fmt.Sprintf("%x", md5.Sum([]byte(val)))
	return s
}

// SaltedHash preprend the secret before hashing input
func SaltedHash(val string, salt ...string) string {
	var s string
	if len(salt) > 0 {
		s = salt[0]
	} else {
		s = GetSecret()
	}
	return hashReport(s + ":" + val)
}

// ScrubYaml scrub the values in yaml
func ScrubYaml(
	data []byte,
	hashFieldNames set.StringSet,
	omitFieldNames set.StringSet,
	salt string,
) (scrubed []byte, err error) {
	mp := make(map[any]any)
	err = yaml.Unmarshal(data, mp)
	if err != nil {
		return nil, err
	}

	smp := scrupMap(mp, hashFieldNames, omitFieldNames, false, false, salt).(map[string]any)
	scrubed, err = yaml.Marshal(smp)
	return
}

// scrupMap scrub the values in map
// for string type, replace as "_" if the field is in the omit list,
// for any other type set as the zero value of the according type if it's in the omit list.
// so configs are ended up with only keys reported, and all field values are hash masked
// or with zero values.
func scrupMap(
	val any,
	hashFieldNames set.StringSet,
	omitFieldNames set.StringSet,
	hash, omit bool,
	salt string,
) any {
	if val == nil {
		return nil
	}

	m, ok := val.(map[any]any)
	if ok {
		ret := make(map[string]any)
		for k, v := range m {
			kk, ok := k.(string)
			if !ok {
				return val
			}
			khash := hash || hashFieldNames.Exist(kk)
			komit := omit || omitFieldNames.Exist(kk)
			ret[kk] = scrupMap(v, hashFieldNames, omitFieldNames, khash, komit, salt)
		}
		return ret
	}

	rv := reflect.ValueOf(val)
	switch rv.Kind() {
	case reflect.Slice:
		var ret []any
		for i := 0; i < rv.Len(); i++ {
			ret = append(ret,
				scrupMap(rv.Index(i).Interface(), hashFieldNames, omitFieldNames, hash, omit, salt))
		}
		return ret
	case reflect.Map:
		ret := make(map[string]any)
		iter := rv.MapRange()
		for iter.Next() {
			k := iter.Key().String()
			v := iter.Value().Interface()
			ret[k] = scrupMap(v, hashFieldNames, omitFieldNames, hash, omit, salt)
		}
		return ret
	case reflect.String:
		if hash && !rv.IsZero() {
			return hashReport(salt + ":" + rv.String())
		}
		if omit {
			return "_"
		}
	}

	if omit {
		return reflect.Zero(rv.Type()).Interface()
	}
	return rv.Interface()
}
