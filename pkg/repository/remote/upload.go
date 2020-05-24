package remote

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"time"

	cjson "github.com/gibson042/canonicaljson-go"
	"github.com/pingcap-incubator/tiup/pkg/repository/v1manifest"
	"github.com/pingcap-incubator/tiup/pkg/utils"
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
	postAddr := fmt.Sprintf("%s/api/v1/tarbal/%s", t.endpoint, t.sha256)
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
	}

	platformStr := fmt.Sprintf("%s/%s", t.os, t.arch)
	if m.Platforms[platformStr] == nil {
		m.Platforms[platformStr] = map[string]v1manifest.VersionItem{}
	}
	m.Platforms[platformStr][t.version] = v1manifest.VersionItem{
		Entry: t.entry,
		URL:   fmt.Sprintf("/%s-%s-%s-%s.tar.gz", t.component, t.version, t.os, t.arch),
		FileHash: v1manifest.FileHash{
			Hashes: map[string]string{
				"sha256": t.sha256,
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
	resp.Body.Close()

	return nil
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
		Name:        t.component,
		Description: t.description,
		Platforms:   make(map[string]map[string]v1manifest.VersionItem),
	}
}
