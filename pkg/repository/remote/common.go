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
	"strings"

	cjson "github.com/gibson042/canonicaljson-go"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
)

func signAndSend(url string, m *v1manifest.Component, key *v1manifest.KeyInfo, options map[string]bool) error {
	id, err := key.ID()
	if err != nil {
		return err
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

	qpairs := []string{}
	for k, v := range options {
		qpairs = append(qpairs, fmt.Sprintf("%s=%t", k, v))
	}
	qstr := ""
	if len(qpairs) > 0 {
		qstr = "?" + strings.Join(qpairs, "&")
	}
	addr := fmt.Sprintf("%s%s", url, qstr)

	resp, err := http.Post(addr, "text/json", bodyBuf)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 300 {
		return nil
	} else if resp.StatusCode == http.StatusConflict {
		return fmt.Errorf("Local component manifest is not new enough, update it first")
	} else if resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("The server refused, make sure you have access to this component")
	}

	buf := new(strings.Builder)
	if _, err := io.Copy(buf, resp.Body); err != nil {
		return err
	}

	return fmt.Errorf("Unknow error from server, response body: %s", buf.String())
}
