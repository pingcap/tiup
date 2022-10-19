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

package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/pingcap/fn"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/repository"
	"github.com/pingcap/tiup/pkg/repository/model"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/pingcap/tiup/server/session"
)

// SignComponent handles requests to re-sign component manifest
func SignComponent(sm session.Manager, mirror repository.Mirror) http.Handler {
	return &componentSigner{sm, mirror}
}

type componentSigner struct {
	sm     session.Manager
	mirror repository.Mirror
}

func (h *componentSigner) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fn.Wrap(h.sign).ServeHTTP(w, r)
}

func buildInfo(r *http.Request, sid string) *model.PublishInfo {
	info := &model.PublishInfo{}

	m := map[string]**bool{
		repository.OptionYanked:     &info.Yank,
		repository.OptionStandalone: &info.Stand,
		repository.OptionHidden:     &info.Hide,
	}

	for k, v := range m {
		f := false
		if query(r, k) == "true" {
			f = true
			*v = &f
		} else if query(r, k) == "false" {
			*v = &f
		}
	}

	return info
}

func (h *componentSigner) sign(r *http.Request, m *v1manifest.RawManifest) (sr *simpleResponse, err statusError) {
	sid := mux.Vars(r)["sid"]
	name := mux.Vars(r)["name"]
	info := buildInfo(r, sid)

	blackList := []string{"root", "index", "snapshot", "timestamp"}
	for _, b := range blackList {
		if name == b {
			return nil, ErrorForbiden
		}
	}

	logprinter.Infof("Sign component manifest for %s, sid: %s", name, sid)
	fileName, readCloser, rErr := h.sm.Read(sid)
	if rErr != nil {
		logprinter.Errorf("Read tar info for component %s, sid: %s", name, sid)
		return nil, ErrorInternalError
	}
	info.ComponentData = &model.TarInfo{
		Reader: readCloser,
		Name:   fileName,
	}

	comp := v1manifest.Component{}
	if err := json.Unmarshal(m.Signed, &comp); err != nil {
		logprinter.Errorf("Unmarshal manifest %s", err.Error())
		return nil, ErrorInvalidManifest
	}

	manifest := &v1manifest.Manifest{
		Signatures: m.Signatures,
		Signed:     &comp,
	}

	switch err := h.mirror.Publish(manifest, info); err {
	case model.ErrorConflict:
		return nil, ErrorManifestConflict
	case model.ErrorWrongSignature:
		return nil, ErrorForbiden
	case model.ErrorWrongChecksum, model.ErrorWrongFileName:
		logprinter.Errorf("Publish component: %s", err.Error())
		return nil, ErrorInvalidTarball
	case nil:
		return nil, nil
	default:
		h.sm.Delete(sid)
		logprinter.Errorf("Publish component: %s", err.Error())
		return nil, ErrorInternalError
	}
}

func query(r *http.Request, q string) string {
	qs := r.URL.Query()[q]

	if len(qs) == 0 {
		return ""
	}

	return qs[0]
}
