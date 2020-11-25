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
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/utils"
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
	reader, err := fs.mirror.Fetch(resource, 0)
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
	resName += ".tar.gz"

	shaReader, err := fs.mirror.Fetch(shaName, 0)
	if err != nil {
		return errors.Trace(err)
	}
	defer shaReader.Close()

	sha1Content, err := ioutil.ReadAll(shaReader)
	if err != nil {
		return errors.Trace(err)
	}

	var tarReader io.ReadCloser
	if err := fs.mirror.Download(resName, targetDir); err != nil {
		return errors.Trace(err)
	}
	tarReader, err = os.OpenFile(filepath.Join(targetDir, resName), os.O_RDONLY, 0)
	if err != nil {
		return errors.Trace(err)
	}
	defer tarReader.Close()

	if utils.CheckSHA(tarReader, string(sha1Content)) != nil {
		return errors.Trace(err)
	}

	if !expand {
		return nil
	}

	tarFile, ok := tarReader.(*os.File)
	if !ok {
		return errors.Errorf("expected file, found %v", tarReader)
	}
	// make sure to read from file start, it was read once in CheckSHA()
	_, err = tarFile.Seek(0, 0)
	if err != nil {
		return errors.Trace(err)
	}

	return utils.Untar(tarFile, targetDir)
}
