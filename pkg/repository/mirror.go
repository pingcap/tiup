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
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cavaliercoder/grab"
	"github.com/pingcap/errors"
)

type (
	// DownloadProgress represents the download progress notifier
	DownloadProgress interface {
		SetTotal(size int64)
		SetCurrent(size int64)
		Finish()
	}

	// MirrorOptions is used to customize the mirror download options
	MirrorOptions struct {
		Progress DownloadProgress
	}

	// Mirror represents a repository mirror, which can be remote HTTP
	// server or a local file system directory
	Mirror interface {
		// Open initialize the mirror.
		Open() error
		// Download fetches a resource to disk.
		Download(resource, targetDir string) error
		// Fetch fetches a resource into memory. The caller must close the returned reader.
		Fetch(resource string) (io.ReadCloser, error)
		// Close closes the mirror and release local stashed files.
		Close() error
	}
)

// NewMirror returns a mirror instance base on the schema of mirror
func NewMirror(mirror string, options MirrorOptions) Mirror {
	if options.Progress == nil {
		options.Progress = &ProgressBar{}
	}
	if strings.HasPrefix(mirror, "http") {
		return &httpMirror{
			server:  mirror,
			options: options,
		}
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
		return errors.Trace(err)
	}
	if !fi.IsDir() {
		return errors.Errorf("local system mirror `%s` should be a directory", l.rootPath)
	}
	return nil
}

// Download implements the Mirror interface
func (l *localFilesystem) Download(resource, targetDir string) error {
	reader, err := l.Fetch(resource)
	if err != nil {
		return errors.Trace(err)
	}
	outPath := filepath.Join(targetDir, resource)
	writer, err := os.OpenFile(outPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return errors.Trace(err)
	}
	_, err = io.Copy(writer, reader)
	return err
}

// Fetch implements the Mirror interface
func (l *localFilesystem) Fetch(resource string) (io.ReadCloser, error) {
	path := filepath.Join(l.rootPath, resource)
	file, err := os.OpenFile(path, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return file, nil
}

// Close implements the Mirror interface
func (l *localFilesystem) Close() error {
	return nil
}

type httpMirror struct {
	server  string
	tmpDir  string
	options MirrorOptions
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

func (l *httpMirror) download(url string, to string) error {
	client := grab.NewClient()
	req, err := grab.NewRequest(to, url)
	if err != nil {
		return errors.Trace(err)
	}

	resp := client.Do(req)

	// start progress output loop
	t := time.NewTicker(time.Millisecond)
	defer t.Stop()

	var progress DownloadProgress
	if strings.Contains(url, ".tar.gz") {
		fmt.Printf("download %s:\n", url)
		progress = l.options.Progress
	} else {
		progress = DisableProgress{}
	}
	progress.SetTotal(resp.Size)

L:
	for {
		select {
		case <-t.C:
			progress.SetCurrent(resp.BytesComplete())
		case <-resp.Done:
			progress.Finish()
			break L
		}
	}

	// check for errors
	if err := resp.Err(); err != nil {
		return errors.Annotatef(err, "download from %s failed", url)
	}

	return nil
}

// Download implements the Mirror interface
func (l *httpMirror) Download(resource, targetDir string) error {
	url := strings.TrimSuffix(l.server, "/") + "/" + resource

	// Force CDN to refresh if the resource name starts with TiupBinaryName.
	if strings.HasPrefix(resource, TiupBinaryName) {
		nano := time.Now().UnixNano()
		url = fmt.Sprintf("%s?v=%d", url, nano)
	}

	return l.download(url, filepath.Join(targetDir, resource))
}

// Fetch implements the Mirror interface
func (l *httpMirror) Fetch(resource string) (io.ReadCloser, error) {
	// FIXME This is inefficient because we write the file to disk, then we read it back in. It would be better
	// for us to read the file straight from memory. This can be done using Request::NoStore and Response::Open.
	err := l.Download(resource, l.tmpDir)
	if err != nil {
		return nil, errors.Trace(err)
	}
	path := filepath.Join(l.tmpDir, resource)
	file, err := os.OpenFile(path, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return file, nil
}

// Close implements the Mirror interface
func (l *httpMirror) Close() error {
	if err := os.RemoveAll(l.tmpDir); err != nil {
		return errors.Trace(err)
	}
	return nil
}
