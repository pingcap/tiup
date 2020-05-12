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
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/pingcap/errors"
)

// IsExist check whether a path is exist
func IsExist(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// IsNotExist check whether a path is not exist
func IsNotExist(path string) bool {
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}

// IsEmptyDir check whether a path is an empty directory
func IsEmptyDir(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

// Untar decompresses the tarball
func Untar(reader io.Reader, to string) error {
	gr, err := gzip.NewReader(reader)
	if err != nil {
		return errors.Trace(err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	decFile := func(hdr *tar.Header) error {
		file := path.Join(to, hdr.Name)
		if dir := filepath.Dir(file); IsNotExist(dir) {
			err := os.MkdirAll(dir, 0755)
			if err != nil {
				return err
			}
		}
		fw, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, hdr.FileInfo().Mode())
		if err != nil {
			return errors.Trace(err)
		}
		defer fw.Close()

		_, err = io.Copy(fw, tr)
		return errors.Trace(err)
	}

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.Trace(err)
		}
		if hdr.FileInfo().IsDir() {
			if err := os.MkdirAll(path.Join(to, hdr.Name), hdr.FileInfo().Mode()); err != nil {
				return errors.Trace(err)
			}
		} else {
			if err := decFile(hdr); err != nil {
				return errors.Trace(err)
			}
		}
	}
	return nil
}

// Copy copy file from src to dst
func Copy(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Close()
}
