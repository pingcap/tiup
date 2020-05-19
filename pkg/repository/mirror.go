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
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cavaliercoder/grab"
	"github.com/pingcap/errors"
)

// ErrNotFound represents the resource not exists.
var ErrNotFound = errors.New("not found")

type (
	// DownloadProgress represents the download progress notifier
	DownloadProgress interface {
		Start(url string, size int64)
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
		// The implementation must return ErrNotFound if the resource not exists.
		Download(resource, targetDir string) error
		// Fetch fetches a resource into memory. The caller must close the returned reader. Id the size of the resource
		// is greater than maxSize, Fetch returns an error. Use maxSize == 0 for no limit.
		// The implementation must return ErrNotFound if the resource not exists.
		Fetch(resource string, maxSize int64) (io.ReadCloser, error)
		// Close closes the mirror and release local stashed files.
		Close() error
	}
)

// NewMirror returns a mirror instance Base on the schema of mirror
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
	reader, err := l.Fetch(resource, 0)
	if err != nil {
		return errors.Trace(err)
	}
	outPath := filepath.Join(targetDir, resource)
	writer, err := os.OpenFile(outPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return errors.Trace(err)
	}
	_, err = io.Copy(writer, reader)
	return err
}

// Fetch implements the Mirror interface
func (l *localFilesystem) Fetch(resource string, maxSize int64) (io.ReadCloser, error) {
	path := filepath.Join(l.rootPath, resource)
	file, err := os.OpenFile(path, os.O_RDONLY, os.ModePerm)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, errors.Trace(err)
	}
	if maxSize > 0 {
		info, err := file.Stat()
		if err != nil {
			return nil, errors.Trace(err)
		}
		if info.Size() > maxSize {
			return nil, errors.Annotatef(err, "local load from %s failed, maximum size exceeded", resource)
		}
	}

	return file, nil
}

// Close implements the Mirror interface
func (l *localFilesystem) Close() error {
	return nil
}

type httpMirror struct {
	server  string
	options MirrorOptions
}

// Open implements the Mirror interface
func (l *httpMirror) Open() error {
	return nil
}

func (l *httpMirror) download(url string, to string, maxSize int64) (io.ReadCloser, error) {
	client := grab.NewClient()
	req, err := grab.NewRequest(to, url)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if len(to) == 0 {
		req.NoStore = true
	}

	resp := client.Do(req)

	// start progress output loop
	t := time.NewTicker(time.Millisecond)
	defer t.Stop()

	var progress DownloadProgress
	if strings.Contains(url, ".tar.gz") {
		progress = l.options.Progress
	} else {
		progress = DisableProgress{}
	}
	progress.Start(url, resp.Size())

L:
	for {
		select {
		case <-t.C:
			if maxSize > 0 && resp.BytesComplete() > maxSize {
				resp.Cancel()
				return nil, errors.Annotatef(err, "download from %s failed, maximum size exceeded", url)
			}
			progress.SetCurrent(resp.BytesComplete())
		case <-resp.Done:
			progress.Finish()
			break L
		}
	}

	// check for errors
	if err := resp.Err(); err != nil {
		if grab.IsStatusCodeError(err) {
			code := err.(grab.StatusCodeError)
			if int(code) == http.StatusNotFound {
				return nil, ErrNotFound
			}
		}
		return nil, errors.Annotatef(err, "download from %s failed", url)
	}
	if maxSize > 0 && resp.BytesComplete() > maxSize {
		return nil, errors.Annotatef(err, "download from %s failed, maximum size exceeded", url)
	}

	return resp.Open()
}

func (l *httpMirror) prepareURL(resource string) string {
	url := strings.TrimSuffix(l.server, "/") + "/" + resource

	// Force CDN to refresh if the resource name starts with TiupBinaryName.
	if strings.HasPrefix(resource, TiupBinaryName) {
		nano := time.Now().UnixNano()
		url = fmt.Sprintf("%s?v=%d", url, nano)
	}

	return url
}

// Download implements the Mirror interface
func (l *httpMirror) Download(resource, targetDir string) error {
	r, err := l.download(l.prepareURL(resource), filepath.Join(targetDir, resource), 0)
	r.Close()
	return err
}

// Fetch implements the Mirror interface
func (l *httpMirror) Fetch(resource string, maxSize int64) (io.ReadCloser, error) {
	return l.download(l.prepareURL(resource), "", maxSize)
}

// Close implements the Mirror interface
func (l *httpMirror) Close() error {
	return nil
}

// MockMirror is a mirror for testing
type MockMirror struct {
	// Resources is a map from resource name to resource content.
	Resources map[string]string
}

// Open implements Mirror.
func (l *MockMirror) Open() error {
	return nil
}

// Download implements Mirror.
func (l *MockMirror) Download(resource, targetDir string) error {
	return errors.New("MockMirror::Download not implemented")
}

// Fetch implements Mirror.
func (l *MockMirror) Fetch(resource string, maxSize int64) (io.ReadCloser, error) {
	content, ok := l.Resources[resource]
	if !ok {
		return nil, ErrNotFound
	}
	if maxSize > 0 && int64(len(content)) > maxSize {
		return nil, fmt.Errorf("Oversized resource %s in mock mirror", resource)
	}
	return ioutil.NopCloser(strings.NewReader(content)), nil
}

// Close implements Mirror.
func (l *MockMirror) Close() error {
	return nil
}
