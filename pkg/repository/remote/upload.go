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

package remote

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	cjson "github.com/gibson042/canonicaljson-go"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/pingcap/tiup/pkg/version"
)

// Transporter defines methods to upload components
type Transporter interface {
	WithOS(os string) Transporter
	WithArch(arch string) Transporter
	WithDesc(desc string) Transporter
	Open(tarbal string) error
	Close() error
	Upload() error
	Sign(key *v1manifest.KeyInfo, m *v1manifest.Component) error
}

type transporter struct {
	tarFile     *os.File
	tarInfo     os.FileInfo
	sha256      string
	sha512      string
	os          string
	arch        string
	entry       string
	component   string
	version     string
	description string
	endpoint    string
}

// New returns a Transporter
func New(endpoint, component, version, entry string) Transporter {
	return &transporter{
		endpoint:  endpoint,
		component: component,
		entry:     entry,
		os:        runtime.GOOS,
		arch:      runtime.GOARCH,
		version:   version,
	}
}

func (t *transporter) WithOS(os string) Transporter {
	t.os = os
	return t
}

func (t *transporter) WithDesc(desc string) Transporter {
	t.description = desc
	return t
}

func (t *transporter) WithArch(arch string) Transporter {
	t.arch = arch
	return t
}

func (t *transporter) Open(tarbal string) error {
	info, err := os.Stat(tarbal)
	if err != nil {
		return err
	}
	t.tarInfo = info

	file, err := os.Open(tarbal)
	if err != nil {
		return err
	}
	t.tarFile = file

	sha256, err := utils.SHA256(file)
	if err != nil {
		file.Close()
		return err
	}
	t.sha256 = sha256

	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}

	sha512, err := utils.SHA512(file)
	if err != nil {
		file.Close()
		return err
	}
	t.sha512 = sha512

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		file.Close()
		return err
	}

	return nil
}

func (t *transporter) Close() error {
	return t.tarFile.Close()
}

func (t *transporter) Upload() error {
	postAddr := fmt.Sprintf("%s/api/v1/tarball/%s", t.endpoint, t.sha256)
	tarbalName := fmt.Sprintf("%s-%s-%s-%s.tar.gz", t.component, t.version, t.os, t.arch)
	resp, err := utils.PostFile(t.tarFile, postAddr, "file", tarbalName)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

func (t *transporter) Sign(key *v1manifest.KeyInfo, m *v1manifest.Component) error {
	id, err := key.ID()
	if err != nil {
		return err
	}

	if m == nil {
		m = t.defaultComponent()
	} else {
		m.Version++
	}

	if strings.Contains(t.version, version.NightlyVersion) {
		m.Nightly = t.version
	}

	platformStr := fmt.Sprintf("%s/%s", t.os, t.arch)
	if m.Platforms[platformStr] == nil {
		m.Platforms[platformStr] = map[string]v1manifest.VersionItem{}
	}
	m.Platforms[platformStr][t.version] = v1manifest.VersionItem{
		Entry:    t.entry,
		Released: time.Now().Format(time.RFC3339),
		URL:      fmt.Sprintf("/%s-%s-%s-%s.tar.gz", t.component, t.version, t.os, t.arch),
		FileHash: v1manifest.FileHash{
			Hashes: map[string]string{
				v1manifest.SHA256: t.sha256,
				v1manifest.SHA512: t.sha512,
			},
			Length: uint(t.tarInfo.Size()),
		},
	}

	payload, err := cjson.Marshal(m)
	if err != nil {
		return err
	}
	sig, err := key.Signature(payload)
	if err != nil {
		return err
	}
	manifest := v1manifest.Manifest{
		Signatures: []v1manifest.Signature{{
			KeyID: id,
			Sig:   sig,
		}},
		Signed: m,
	}

	payload, err = json.Marshal(manifest)
	if err != nil {
		return err
	}
	bodyBuf := bytes.NewBuffer(payload)
	addr := fmt.Sprintf("%s/api/v1/component/%s/%s", t.endpoint, t.sha256, t.component)
	resp, err := http.Post(addr, "text/json", bodyBuf)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 300 {
		return nil
	} else if resp.StatusCode == http.StatusConflict {
		return fmt.Errorf("Local manifest for component %s is not new enough, update it first", t.component)
	} else if resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("The server refused, make sure you have access to this component: %s", t.component)
	}

	buf := new(strings.Builder)
	if _, err := io.Copy(buf, resp.Body); err != nil {
		return err
	}

	return fmt.Errorf("Unknow error from server, response body: %s", buf.String())
}

func (t *transporter) defaultComponent() *v1manifest.Component {
	initTime := time.Now()
	return &v1manifest.Component{
		SignedBase: v1manifest.SignedBase{
			Ty:          v1manifest.ManifestTypeComponent,
			SpecVersion: v1manifest.CurrentSpecVersion,
			Expires:     initTime.Add(v1manifest.ManifestsConfig[v1manifest.ManifestTypeComponent].Expire).Format(time.RFC3339),
			Version:     1, // initial repo starts with version 1
		},
		ID:          t.component,
		Description: t.description,
		Platforms:   make(map[string]map[string]v1manifest.VersionItem),
	}
}
