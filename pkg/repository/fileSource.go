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
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"github.com/pingcap-incubator/tiup/pkg/utils"
	"github.com/pingcap/errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// fileSource is a high-level interface for providing file-like objects, either as objects in memory or purely on disk.
type fileSource interface {
	open() error
	close() error
	downloadJSON(resource string, result interface{}) error
	downloadTarFile(targetDir, resName string, expand bool) error
}

type mirrorSource struct {
	mirror Mirror
}

func (fs *mirrorSource) open() error {
	return fs.mirror.Open()
}

func (fs *mirrorSource) close() error {
	return fs.mirror.Close()
}

// downloadJSON fetches a resource from m and parses it as json.
func (fs *mirrorSource) downloadJSON(resource string, result interface{}) error {
	reader, err := fs.mirror.Fetch(resource)
	if err != nil {
		return errors.Trace(err)
	}
	defer reader.Close()

	err = json.NewDecoder(reader).Decode(result)
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}

// downloadTarFile downloads a tar file from our mirror and checks its sha1 hash.
func (fs *mirrorSource) downloadTarFile(targetDir, resName string, expand bool) error {
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return errors.Trace(err)
	}

	shaName := resName + ".sha1"
	resName = resName + ".tar.gz"

	shaReader, err := fs.mirror.Fetch(shaName)
	if err != nil {
		return errors.Trace(err)
	}
	defer shaReader.Close()

	sha1Content, err := ioutil.ReadAll(shaReader)
	if err != nil {
		return errors.Trace(err)
	}

	var tarReader io.ReadCloser
	if expand {
		tarReader, err = fs.mirror.Fetch(resName)
	} else {
		if err := fs.mirror.Download(resName, targetDir); err != nil {
			return errors.Trace(err)
		}
		tarReader, err = os.OpenFile(filepath.Join(targetDir, resName), os.O_RDONLY, 0)
	}
	if err != nil {
		return errors.Trace(err)
	}
	defer tarReader.Close()

	if checkSHA(tarReader, string(sha1Content)) != nil {
		return errors.Trace(err)
	}

	if !expand {
		return nil
	}

	tarFile, ok := tarReader.(*os.File)
	if !ok {
		return errors.Errorf("Expected file, found %v", tarReader)
	}
	_, err = tarFile.Seek(0, 0)
	if err != nil {
		return errors.Trace(err)
	}

	return utils.Untar(tarFile, targetDir)
}

func checkSHA(reader io.Reader, sha string) error {
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
