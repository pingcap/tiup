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
	"bytes"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cavaliergopher/grab/v3"
	"github.com/google/uuid"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/crypto/rand"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/repository/model"
	"github.com/pingcap/tiup/pkg/repository/store"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/pingcap/tiup/pkg/utils/mock"
	"github.com/pingcap/tiup/pkg/version"
)

const (
	// OptionYanked is the key that represents a component is yanked or not
	OptionYanked = "yanked"
	// OptionStandalone is the key that represents a component is standalone or not
	OptionStandalone = "standalone"
	// OptionHidden is the key that represents a component is hidden or not
	OptionHidden = "hidden"
)

// predefined errors
var (
	ErrNotFound       = stderrors.New("not found") // resource does not exists
	ErrManifestTooOld = stderrors.New("component manifest is too old, update it before publish")
)

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
		Upstream string
		KeyDir   string
	}

	// Mirror represents a repository mirror, which can be remote HTTP
	// server or a local file system directory
	Mirror interface {
		model.Backend
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
	return &localFilesystem{rootPath: mirror, keyDir: options.KeyDir, upstream: options.Upstream}
}

type localFilesystem struct {
	rootPath string
	keyDir   string
	upstream string
	keys     map[string]*v1manifest.KeyInfo
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

	if l.keyDir == "" {
		l.keyDir = path.Join(l.rootPath, "keys")
	}
	if utils.IsNotExist(l.keyDir) {
		return nil
	}
	return l.loadKeys()
}

// load mirror keys
func (l *localFilesystem) loadKeys() error {
	l.keys = make(map[string]*v1manifest.KeyInfo)
	return filepath.Walk(l.keyDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return errors.Annotate(err, "open file while loadKeys")
		}
		defer f.Close()

		ki := v1manifest.KeyInfo{}
		if err := json.NewDecoder(f).Decode(&ki); err != nil {
			return errors.Annotate(err, "decode key")
		}

		id, err := ki.ID()
		if err != nil {
			return err
		}

		l.keys[id] = &ki
		return nil
	})
}

// Publish implements the model.Backend interface
func (l *localFilesystem) Publish(manifest *v1manifest.Manifest, info model.ComponentInfo) error {
	txn, err := store.New(l.rootPath, l.upstream).Begin()
	if err != nil {
		return err
	}

	if err := model.New(txn, l.keys).Publish(manifest, info); err != nil {
		_ = txn.Rollback()
		return err
	}

	return nil
}

// Grant implements the model.Backend interface
func (l *localFilesystem) Grant(id, name string, key *v1manifest.KeyInfo) error {
	txn, err := store.New(l.rootPath, l.upstream).Begin()
	if err != nil {
		return err
	}

	if err := model.New(txn, l.keys).Grant(id, name, key); err != nil {
		_ = txn.Rollback()
		return err
	}

	return nil
}

// Rotate implements the model.Backend interface
func (l *localFilesystem) Rotate(m *v1manifest.Manifest) error {
	txn, err := store.New(l.rootPath, l.upstream).Begin()
	if err != nil {
		return err
	}

	if err := model.New(txn, l.keys).Rotate(m); err != nil {
		_ = txn.Rollback()
		return err
	}

	return nil
}

// Download implements the Mirror interface
func (l *localFilesystem) Download(resource, targetDir string) error {
	reader, err := l.Fetch(resource, 0)
	if err != nil {
		return errors.Trace(err)
	}
	defer reader.Close()

	if err := os.MkdirAll(targetDir, 0755); err != nil {
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
	defer writer.Close()

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
		logprinter.Verbose("Download resource %s in %s", url, time.Since(start))
	}(time.Now())

	client := grab.NewClient()
	client.UserAgent = fmt.Sprintf("tiup/%s", version.NewTiUPVersion().SemVer())
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
	// Force CDN to refresh if the resource name starts with TiUPBinaryName.
	if strings.HasPrefix(resource, TiUPBinaryName) {
		nano := time.Now().UnixNano()
		url = fmt.Sprintf("%s?v=%d", url, nano)
	}

	return url
}

// Grant implements the model.Backend interface
func (l *httpMirror) Grant(id, name string, key *v1manifest.KeyInfo) error {
	return errors.Errorf("cannot add a user for a remote mirror, please set your mirror to a local directory")
}

// Rotate implements the model.Backend interface
func (l *httpMirror) Rotate(m *v1manifest.Manifest) error {
	rotateAddr := fmt.Sprintf("%s/api/v1/rotate", l.Source())
	data, err := json.Marshal(m)
	if err != nil {
		return errors.Annotate(err, "marshal root manifest")
	}

	client := http.Client{Timeout: time.Minute}
	resp, err := client.Post(rotateAddr, "text/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 300 {
		return nil
	}
	switch resp.StatusCode {
	case http.StatusConflict:
		return errors.Errorf("The manifest has been modified after you fetched it, please try again")
	case http.StatusBadRequest:
		return errors.Errorf("The server rejected the manifest, please check if it's a valid root manifest")
	default:
		buf := new(strings.Builder)
		if _, err := io.Copy(buf, resp.Body); err != nil {
			return err
		}

		return fmt.Errorf("Unknow error from server, response code: %d response body: %s", resp.StatusCode, buf.String())
	}
}

// Publish implements the model.Backend interface
func (l *httpMirror) Publish(manifest *v1manifest.Manifest, info model.ComponentInfo) error {
	sid := uuid.New().String()

	if info.Filename() != "" {
		tarAddr := fmt.Sprintf("%s/api/v1/tarball/%s", l.Source(), sid)
		resp, err := utils.PostFile(info, tarAddr, "file", info.Filename())
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 300 {
			return errors.Errorf("error on uplaod tarbal, server returns %d", resp.StatusCode)
		}
	}

	payload, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	bodyBuf := bytes.NewBuffer(payload)
	q := url.Values{}
	if info.Yanked() != nil {
		q.Set(OptionYanked, fmt.Sprintf("%t", *info.Yanked()))
	}
	if info.Standalone() != nil {
		q.Set(OptionStandalone, fmt.Sprintf("%t", *info.Standalone()))
	}
	if info.Hidden() != nil {
		q.Set(OptionHidden, fmt.Sprintf("%t", *info.Hidden()))
	}
	qstr := ""
	if len(q) > 0 {
		qstr = "?" + q.Encode()
	}
	manifestAddr := fmt.Sprintf("%s/api/v1/component/%s/%s%s", l.Source(), sid, manifest.Signed.(*v1manifest.Component).ID, qstr)

	client := http.Client{Timeout: time.Minute}
	resp, err := client.Post(manifestAddr, "text/json", bodyBuf)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 300 {
		return nil
	}
	switch resp.StatusCode {
	case http.StatusConflict:
		return ErrManifestTooOld
	case http.StatusForbidden:
		return errors.Errorf("The server refused, make sure you have access to this component")
	default:
		buf := new(strings.Builder)
		if _, err := io.Copy(buf, resp.Body); err != nil {
			return err
		}

		return fmt.Errorf("Unknow error from server, response code: %d response body: %s", resp.StatusCode, buf.String())
	}
}

func (l *httpMirror) isRetryable(err error) bool {
	retryableList := []string{
		"unexpected EOF",
		"stream error",
		"server returned 502 Bad Gateway",
	}

	for _, text := range retryableList {
		if strings.Contains(strings.ToLower(err.Error()), strings.ToLower(text)) {
			return true
		}
	}
	return false
}

// Download implements the Mirror interface
func (l *httpMirror) Download(resource, targetDir string) error {
	tmpFilePath := filepath.Join(l.tmpDir, resource)
	dstFilePath := filepath.Join(targetDir, resource)
	// downloaded file is stored in a temp directory and the temp directory is
	// deleted at Close(), in this way an interrupted download won't remain
	// any partial file on the disk
	var err error
	_ = utils.Retry(func() error {
		var r io.ReadCloser
		if err != nil && l.isRetryable(err) {
			logprinter.Warnf("failed to download %s(%s), retrying...", resource, err.Error())
		}
		if r, err = l.download(l.prepareURL(resource), tmpFilePath, 0); err != nil {
			if l.isRetryable(err) {
				return err
			}
			// Abort retry
			return nil
		}
		return r.Close()
	}, utils.RetryOption{
		Timeout:  time.Hour,
		Attempts: 3,
	})
	if err != nil {
		return err
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return errors.Trace(err)
	}
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
	content, ok := l.Resources[resource]
	if !ok {
		return errors.Annotatef(ErrNotFound, "resource %s", resource)
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}
	target := filepath.Join(targetDir, resource)

	file, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write([]byte(content))
	return err
}

// Grant implements the model.Backend interface
func (l *MockMirror) Grant(id, name string, key *v1manifest.KeyInfo) error {
	return nil
}

// Rotate implements the model.Backend interface
func (l *MockMirror) Rotate(m *v1manifest.Manifest) error {
	return nil
}

// Publish implements the Mirror interface
func (l *MockMirror) Publish(manifest *v1manifest.Manifest, info model.ComponentInfo) error {
	// Mock point for unit test
	if fn := mock.On("Publish"); fn != nil {
		fn.(func(*v1manifest.Manifest, model.ComponentInfo))(manifest, info)
	}
	return nil
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
	return io.NopCloser(strings.NewReader(content)), nil
}

// Close implements Mirror.
func (l *MockMirror) Close() error {
	return nil
}
