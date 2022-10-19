// Copyright 2021 PingCAP, Inc.
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

package checkpoint

import (
	"reflect"

	"github.com/pingcap/tiup/pkg/set"
)

// FieldSet is an ordered set stores fields and comparators, it's []CheckField
type FieldSet struct {
	set.AnySet
}

// Slice returns CheckField slice for iteration
func (fs *FieldSet) Slice() []CheckField {
	slice := []CheckField{}
	for _, f := range fs.AnySet.Slice() {
		slice = append(slice, f.(CheckField))
	}
	return slice
}

// CheckField is a field that should be checked
type CheckField struct {
	field string
	eq    func(any, any) bool
}

// Field returns new CheckField
func Field(name string, eq func(any, any) bool) CheckField {
	return CheckField{name, eq}
}

// Register register FieldSet to global for latter comparing
// If there are two fields with the same name, the first one will
// be used
func Register(fields ...CheckField) FieldSet {
	s := set.NewAnySet(func(a, b any) bool {
		return a.(CheckField).field == b.(CheckField).field
	})

	fields = append(fields, Field(hashKey, reflect.DeepEqual), Field(funcKey, reflect.DeepEqual))
	for _, f := range fields {
		s.Insert(f)
	}

	return FieldSet{*s}
}
