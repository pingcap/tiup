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
	"crypto/sha1"
	"encoding/hex"
	"io"
	"strings"

	"github.com/pingcap/errors"
)

// CheckSHA returns error if the hash of reader content mismatches `sha`
func CheckSHA(reader io.Reader, sha string) error {
	sha1Writer := sha1.New()
	if _, err := io.Copy(sha1Writer, reader); err != nil {
		return errors.Trace(err)
	}

	checksum := hex.EncodeToString(sha1Writer.Sum(nil))
	if checksum != strings.TrimSpace(sha) {
		return errors.Errorf("checksum mismatch, expect: %v, got: %v", sha, checksum)
	}
	return nil
}
