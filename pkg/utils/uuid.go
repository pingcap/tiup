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

package utils

import (
	"encoding/hex"
	"unicode"

	"github.com/google/uuid"
)

// UUID is an unique ID generator compatiable with legancy TiOps implementation,
// it converts the input node to a reproduciable 8-bytes string.
func UUID(node string) string {
	var buf [32]byte

	_uuid := uuid.NewSHA1(uuid.NameSpaceDNS, []byte(node))

	hex.Encode(buf[:], _uuid[:])
	_id := buf[len(buf)-8:]

	// make sure the first char is letter
	if unicode.IsDigit(rune(_id[0])) {
		_id[0] += 49
	}

	return string(_id)
}
