// Copyright 2026 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package store

import (
	"io"
	"os"
	"strings"

	"github.com/pingcap/errors"
)

func validateFilename(name string) error {
	// Transactions contain flat files, never paths. Do not clean attacker input:
	// normalizing it would hide the attempted traversal from other operations.
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\\x00") {
		return errors.Errorf("invalid repository filename %q", name)
	}
	return nil
}

func openLocalFile(root, name string, flag int) (*os.File, error) {
	if err := validateFilename(name); err != nil {
		return nil, err
	}
	dir, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	// Repository entries must be regular files, not symlinks (including dangling
	// or in-tree links). Root also confines the open if a link changes concurrently.
	fi, err := dir.Lstat(name)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err == nil && !fi.Mode().IsRegular() {
		return nil, errors.Errorf("repository entry %q is not a regular file", name)
	}
	return dir.OpenFile(name, flag, 0644)
}

func statLocalFile(root, name string) (os.FileInfo, error) {
	file, err := openLocalFile(root, name, os.O_RDONLY)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return file.Stat()
}

func copyLocalFile(src, dst, name string) error {
	reader, err := openLocalFile(src, name, os.O_RDONLY)
	if err != nil {
		return err
	}
	defer reader.Close()
	fi, err := reader.Stat()
	if err != nil {
		return err
	}
	writer, err := openLocalFile(dst, name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return err
	}
	defer writer.Close()
	if _, err := io.Copy(writer, reader); err != nil {
		return err
	}
	if err := writer.Chmod(fi.Mode()); err != nil {
		return err
	}
	written, err := writer.Stat()
	if err != nil {
		return err
	}
	// Conflict detection relies on committed files being at least as new as
	// their staged copies, including on filesystems with coarse timestamps.
	if written.ModTime().Before(fi.ModTime()) {
		dir, err := os.OpenRoot(dst)
		if err != nil {
			return err
		}
		defer dir.Close()
		if err := dir.Chtimes(name, fi.ModTime(), fi.ModTime()); err != nil {
			return err
		}
	}
	return writer.Close()
}
