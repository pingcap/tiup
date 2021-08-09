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
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"io"
	"strings"

	"github.com/pingcap/errors"
)

// CheckSHA256 returns an error if the hash of reader mismatches `sha`
func CheckSHA256(reader io.Reader, sha string) error {
	shaWriter := sha256.New()
	if _, err := io.Copy(shaWriter, reader); err != nil {
		return errors.Trace(err)
	}

	checksum := hex.EncodeToString(shaWriter.Sum(nil))
	if checksum != strings.TrimSpace(sha) {
		return &HashValidationErr{
			cipher: "sha256",
			expect: sha,
			actual: checksum,
		}
	}
	return nil
}

// SHA256 returns the hash of reader
func SHA256(reader io.Reader) (string, error) {
	shaWriter := sha256.New()
	if _, err := io.Copy(shaWriter, reader); err != nil {
		return "", errors.Trace(err)
	}

	checksum := hex.EncodeToString(shaWriter.Sum(nil))
	return checksum, nil
}

// SHA512 returns the hash of reader
func SHA512(reader io.Reader) (string, error) {
	shaWriter := sha512.New()
	if _, err := io.Copy(shaWriter, reader); err != nil {
		return "", errors.Trace(err)
	}

	checksum := hex.EncodeToString(shaWriter.Sum(nil))
	return checksum, nil
}
