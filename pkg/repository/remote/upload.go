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
	"github.com/juju/errors"
	ru "github.com/pingcap/tiup/pkg/repository/utils"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/pingcap/tiup/pkg/version"
)

// Transporter defines methods to upload components
type Transporter interface {
	WithOS(os string) Transporter
	WithArch(arch string) Transporter
	WithDesc(desc string) Transporter
	Open(tarball string) error
	Close() error
	Upload() error
	Sign(key *v1manifest.KeyInfo, m *v1manifest.Component) error
}

type transporter struct {
	tarFile     *os.File
	os          string
	arch        string
	entry       string
	component   string
	version     string
	description string
	endpoint    string
	filehash    v1manifest.FileHash
}

// New returns a Transporter
func New(endpoint, component, version, entry string) Transporter {
	return &transporter{
		endpoint:  strings.TrimSuffix(endpoint, "/"),
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

func (t *transporter) Open(tarball string) error {
	hashes, length, err := ru.HashFile(tarball)
	if err != nil {
		return errors.Trace(err)
	}

	t.filehash = v1manifest.FileHash{
		Hashes: hashes,
		Length: uint(length),
	}

	file, err := os.Open(tarball)
	if err != nil {
		return err
	}
	t.tarFile = file

	return nil
}

func (t *transporter) Close() error {
	return t.tarFile.Close()
}

func (t *transporter) Upload() error {
	sha256 := t.filehash.Hashes[v1manifest.SHA256]
	if sha256 == "" {
		return errors.New("sha256 not found for tarball")
	}
	postAddr := fmt.Sprintf("%s/api/v1/tarball/%s", t.endpoint, sha256)
	tarballName := fmt.Sprintf("%s-%s-%s-%s.tar.gz", t.component, t.version, t.os, t.arch)
	resp, err := utils.PostFile(t.tarFile, postAddr, "file", tarballName)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

func (t *transporter) Sign(key *v1manifest.KeyInfo, m *v1manifest.Component) error {
	sha256 := t.filehash.Hashes[v1manifest.SHA256]
	if sha256 == "" {
		return errors.New("sha256 not found for tarball")
	}
	id, err := key.ID()
	if err != nil {
		return err
	}

	initTime := time.Now()
	if m == nil {
		m = t.defaultComponent(initTime)
	} else {
		v1manifest.RenewManifest(m, initTime)
		m.Version++
	}

	if strings.Contains(t.version, version.NightlyVersion) {
		m.Nightly = t.version
	}
	// Remove history nightly
	for plat := range m.Platforms {
		for ver := range m.Platforms[plat] {
			if strings.Contains(ver, version.NightlyVersion) && ver != m.Nightly {
				delete(m.Platforms[plat], ver)
			}
		}
	}

	platformStr := fmt.Sprintf("%s/%s", t.os, t.arch)
	if m.Platforms[platformStr] == nil {
		m.Platforms[platformStr] = map[string]v1manifest.VersionItem{}
	}
	m.Platforms[platformStr][t.version] = v1manifest.VersionItem{
		Entry:    t.entry,
		Released: initTime.Format(time.RFC3339),
		URL:      fmt.Sprintf("/%s-%s-%s-%s.tar.gz", t.component, t.version, t.os, t.arch),
		FileHash: t.filehash,
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
	addr := fmt.Sprintf("%s/api/v1/component/%s/%s", t.endpoint, sha256, t.component)
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

func (t *transporter) defaultComponent(initTime time.Time) *v1manifest.Component {
	return v1manifest.NewComponent(t.component, t.description, initTime)
}
