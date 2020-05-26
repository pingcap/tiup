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

package repository

import (
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"

	cjson "github.com/gibson042/canonicaljson-go"
	"github.com/pingcap-incubator/tiup/pkg/repository/v1manifest"
)

// HashFile returns the sha256/sha512 hashes and the file length of specific file
func HashFile(srcDir, filename string) (map[string]string, int64, error) {
	path := filepath.Join(srcDir, filename)
	s256 := sha256.New()
	s512 := sha512.New()
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer file.Close()

	n, err := io.Copy(io.MultiWriter(s256, s512), file)

	hashes := map[string]string{
		v1manifest.SHA256: hex.EncodeToString(s256.Sum(nil)),
		v1manifest.SHA512: hex.EncodeToString(s512.Sum(nil)),
	}
	return hashes, n, err
}

// HashManifest returns the sha256/sha512 hashes and the file length of specific manifest
func HashManifest(m *v1manifest.Manifest) (map[string]string, uint, error) {
	bytes, err := cjson.Marshal(m)
	if err != nil {
		return nil, 0, err
	}

	s256 := sha256.Sum256(bytes)
	s512 := sha512.Sum512(bytes)

	return map[string]string{
		v1manifest.SHA256: hex.EncodeToString(s256[:]),
		v1manifest.SHA512: hex.EncodeToString(s512[:]),
	}, uint(len(bytes)), nil
}
