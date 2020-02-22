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
	"encoding/hex"
)

// ValidateSHA256 generate SHA256 checksum of a file and compare with given value
func ValidateSHA256(file, checksum string) (bool, error) {
	// always return true if validating is not needed
	if checksum == "SKIP" {
		return true, nil
	}

	hash, err := calFileSHA256(file)
	if err != nil {
		return false, err
	}
	return hash == checksum, nil
}

func calFileSHA256(file string) (string, error) {
	f, err := ReadFile(file)
	if err != nil {
		return "", err
	}

	hashWriter := sha256.New()
	if _, err := hashWriter.Write(f); err != nil {
		return "", err
	}

	return hex.EncodeToString(hashWriter.Sum(nil)), nil
}
