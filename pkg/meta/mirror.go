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

package meta

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cavaliercoder/grab"
	"github.com/cheggaaa/pb"
	"github.com/pingcap-incubator/tiup/pkg/utils"
	"github.com/pingcap/errors"
)

// Mirror represents a repository mirror, which can be remote HTTP
// server or a local file system directory
type Mirror interface {
	// Open initialize the mirror
	Open() error
	// Fetch fetches a resource
	Fetch(resource string) (path string, err error)
	// Close closes the mirror and release local stashed files
	Close() error
}

// NewMirror returns a mirror instance base on the schema of mirror
func NewMirror(mirror string) Mirror {
	if strings.HasPrefix(mirror, "http") {
		return &httpMirror{server: mirror}
	}
	return &localFilesystem{rootPath: mirror}
}

type localFilesystem struct {
	rootPath string
}

// Open implements the Mirror interface
func (l *localFilesystem) Open() error {
	fi, err := os.Stat(l.rootPath)
	if err != nil {
		errors.Trace(err)
	}
	if !fi.IsDir() {
		return errors.Errorf("local system mirror `%s` should be a directory", l.rootPath)
	}
	return nil
}

// Fetch implements the Mirror interface
func (l *localFilesystem) Fetch(resource string) (path string, err error) {
	path = filepath.Join(l.rootPath, resource)
	if utils.IsNotExist(path) {
		return "", errors.Errorf("resource `%s` not found", resource)
	}
	return path, nil
}

// Close implements the Mirror interface
func (l *localFilesystem) Close() error {
	return nil
}

type httpMirror struct {
	server string
	tmpDir string
}

// Open implements the Mirror interface
func (l *httpMirror) Open() error {
	tmpDir := filepath.Join(os.TempDir(), strconv.Itoa(rand.Int()))
	if err := os.MkdirAll(tmpDir, os.ModePerm); err != nil {
		return errors.Trace(err)
	}
	l.tmpDir = tmpDir
	return nil
}

func (l *httpMirror) download(url string, to string) (string, error) {
	client := grab.NewClient()
	req, err := grab.NewRequest(to, url)
	if err != nil {
		return "", errors.Trace(err)
	}

	resp := client.Do(req)

	// start progress output loop
	t := time.NewTicker(time.Millisecond)
	defer t.Stop()

	var bar *pb.ProgressBar
	if strings.HasSuffix(url, ".tar.gz") {
		fmt.Printf("download %s:\n", url)
		bar = pb.StartNew(int(resp.Size))
	}
L:
	for {
		select {
		case <-t.C:
			if bar != nil {
				bar.SetCurrent(resp.BytesComplete())
			}
		case <-resp.Done:
			if bar != nil {
				bar.SetCurrent(bar.Total())
				bar.Finish()
			}
			break L
		}
	}

	// check for errors
	if err := resp.Err(); err != nil {
		return "", errors.Trace(err)
	}

	return resp.Filename, nil
}

// Fetch implements the Mirror interface
func (l *httpMirror) Fetch(resource string) (path string, err error) {
	// refresh CDN forcely if the resource is start with `tiup`
	nano := time.Now().UnixNano()
	url := strings.TrimSuffix(l.server, "/") + "/" + resource
	if strings.HasPrefix(resource, "tiup") {
		url = fmt.Sprintf("%s?v=%d", url, nano)
	}
	tmp := filepath.Join(l.tmpDir, strconv.Itoa(int(nano)))
	return l.download(url, tmp)
}

// Close implements the Mirror interface
func (l *httpMirror) Close() error {
	if err := os.RemoveAll(l.tmpDir); err != nil {
		return errors.Trace(err)
	}
	return nil
}
