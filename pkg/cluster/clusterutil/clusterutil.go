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

package clusterutil

import (
	"path/filepath"
	"strings"
)

// Abs returns the absolute path
func Abs(user, path string) string {
	if !strings.HasPrefix(path, "/") {
		return filepath.Join("/home", user, path)
	}
	return path
}

// MultiDirAbs returns the absolute path for multi-dir separated by comma
func MultiDirAbs(user, paths string) []string {
	var dirs []string
	for _, path := range strings.Split(paths, ",") {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		dirs = append(dirs, Abs(user, path))
	}
	return dirs
}
