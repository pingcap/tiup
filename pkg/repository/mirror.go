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
	stderrors "errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cavaliercoder/grab"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/pingcap/tiup/pkg/verbose"
)

// ErrNotFound represents the resource not exists.
var ErrNotFound = stderrors.New("not found")

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
		// Source returns the address of the mirror
		Source() string
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

// Source implements the Mirror interface
func (l *localFilesystem) Source() string {
	return l.rootPath
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
			return errors.Annotatef(ErrNotFound, "resource %s", resource)
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
			return nil, errors.Annotatef(ErrNotFound, "resource %s", resource)
		}
		return nil, errors.Trace(err)
	}
	if maxSize > 0 {
		info, err := file.Stat()
		if err != nil {
			return nil, errors.Trace(err)
		}

		if info.Size() > maxSize {
			return nil, errors.Errorf("local load from %s failed, maximum size exceeded, file size: %d, max size: %d", resource, info.Size(), maxSize)
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
	tmpDir  string
	options MirrorOptions
}

// Source implements the Mirror interface
func (l *httpMirror) Source() string {
	return l.server
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

func (l *httpMirror) download(url string, to string, maxSize int64) (io.ReadCloser, error) {
	defer func(start time.Time) {
		verbose.Log("Download resource %s in %s", url, time.Since(start))
	}(time.Now())

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
				_ = resp.Cancel()
				return nil, errors.Errorf("download from %s failed, resp size %d exceeds maximum size %d", url, resp.BytesComplete(), maxSize)
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
				return nil, errors.Annotatef(ErrNotFound, "url %s", url)
			}
		}
		return nil, errors.Annotatef(err, "download from %s failed", url)
	}
	if maxSize > 0 && resp.BytesComplete() > maxSize {
		return nil, errors.Errorf("download from %s failed, resp size %d exceeds maximum size %d", url, resp.BytesComplete(), maxSize)
	}

	return resp.Open()
}

func (l *httpMirror) prepareURL(resource string) string {
	url := strings.TrimSuffix(l.server, "/") + "/" + strings.TrimPrefix(resource, "/")
	// Force CDN to refresh if the resource name starts with TiupBinaryName.
	if strings.HasPrefix(resource, TiupBinaryName) {
		nano := time.Now().UnixNano()
		url = fmt.Sprintf("%s?v=%d", url, nano)
	}

	return url
}

// Download implements the Mirror interface
func (l *httpMirror) Download(resource, targetDir string) error {
	tmpFilePath := filepath.Join(l.tmpDir, resource)
	dstFilePath := filepath.Join(targetDir, resource)
	// downloaded file is stored in a temp directory and the temp directory is
	// deleted at Close(), in this way an interrupted download won't remain
	// any partial file on the disk
	r, err := l.download(l.prepareURL(resource), tmpFilePath, 0)
	if err != nil {
		return errors.Trace(err)
	}
	defer r.Close()
	return utils.Move(tmpFilePath, dstFilePath)
}

// Fetch implements the Mirror interface
func (l *httpMirror) Fetch(resource string, maxSize int64) (io.ReadCloser, error) {
	return l.download(l.prepareURL(resource), "", maxSize)
}

// Close implements the Mirror interface
func (l *httpMirror) Close() error {
	if err := os.RemoveAll(l.tmpDir); err != nil {
		return errors.Trace(err)
	}
	return nil
}

// MockMirror is a mirror for testing
type MockMirror struct {
	// Resources is a map from resource name to resource content.
	Resources map[string]string
}

// Source implements the Mirror interface
func (l *MockMirror) Source() string {
	return "mock"
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
		return nil, errors.Annotatef(ErrNotFound, "resource %s", resource)
	}
	if maxSize > 0 && int64(len(content)) > maxSize {
		return nil, fmt.Errorf("oversized resource %s in mock mirror %v > %v", resource, len(content), maxSize)
	}
	return ioutil.NopCloser(strings.NewReader(content)), nil
}

// Close implements Mirror.
func (l *MockMirror) Close() error {
	return nil
}
