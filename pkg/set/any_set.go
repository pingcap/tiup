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

package set

// AnySet is a set stores any
type AnySet struct {
	eq    func(a any, b any) bool
	slice []any
}

// NewAnySet builds a AnySet
func NewAnySet(eq func(a any, b any) bool, aa ...any) *AnySet {
	slice := []any{}
out:
	for _, a := range aa {
		for _, b := range slice {
			if eq(a, b) {
				continue out
			}
		}
		slice = append(slice, a)
	}
	return &AnySet{eq, slice}
}

// Exist checks whether `val` exists in `s`.
func (s *AnySet) Exist(val any) bool {
	for _, a := range s.slice {
		if s.eq(a, val) {
			return true
		}
	}
	return false
}

// Insert inserts `val` into `s`.
func (s *AnySet) Insert(val any) {
	if !s.Exist(val) {
		s.slice = append(s.slice, val)
	}
}

// Intersection returns the intersection of two sets
func (s *AnySet) Intersection(rhs *AnySet) *AnySet {
	newSet := NewAnySet(s.eq)
	for elt := range rhs.slice {
		if s.Exist(elt) {
			newSet.Insert(elt)
		}
	}
	return newSet
}

// Remove removes `val` from `s`
func (s *AnySet) Remove(val any) {
	for i, a := range s.slice {
		if s.eq(a, val) {
			s.slice = append(s.slice[:i], s.slice[i+1:]...)
			return
		}
	}
}

// Difference returns the difference of two sets
func (s *AnySet) Difference(rhs *AnySet) *AnySet {
	newSet := NewAnySet(s.eq)
	diffSet := NewAnySet(s.eq, rhs.slice...)
	for elt := range s.slice {
		if !diffSet.Exist(elt) {
			newSet.Insert(elt)
		}
	}
	return newSet
}

// Slice converts the set to a slice
func (s *AnySet) Slice() []any {
	return append([]any{}, s.slice...)
}
