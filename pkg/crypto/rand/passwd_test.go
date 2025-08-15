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

package rand

import (
	"testing"
)

func TestPasswd(t *testing.T) {
	for range 100 {
		l := Intn(64)
		if l < 8 { // make sure it's greater than 8
			l += 8
		}
		t.Logf("generating random password of length %d", l)
		p, e := Password(l)
		if e != nil {
			t.Error(e)
		}
		t.Log(p)
		if len(p) != l {
			t.Fail()
		}
	}
}
