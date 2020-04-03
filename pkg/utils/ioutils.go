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
	"fmt"
	"io"
	"os"
	"path"
)

// CreateDir creates the directory if it not exists.
func CreateDir(path string) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return os.MkdirAll(path, 0755)
		}
		return err
	}
	return nil
}

// CopyFile copies a file from src to dst
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		return fmt.Errorf("destination path %s already exist", dst)
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

// Move moves a file or directory from src to dst
func Move(src, dst string) error {
	if src == "" || dst == "" {
		return fmt.Errorf("path can not be empty")
	}

	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		return fmt.Errorf("destination path %s already exist", dst)
	}
	if err := CreateDir(path.Dir(dst)); err != nil {
		return err
	}

	return os.Rename(src, dst)
}
