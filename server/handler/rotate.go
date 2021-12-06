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

	"github.com/pingcap/fn"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/repository"
	"github.com/pingcap/tiup/pkg/repository/model"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
)

// RotateRoot handles requests to re-sign root manifest
func RotateRoot(mirror repository.Mirror) http.Handler {
	return &rootSigner{mirror}
}

type rootSigner struct {
	mirror repository.Mirror
}

func (h *rootSigner) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fn.Wrap(h.sign).ServeHTTP(w, r)
}

func (h *rootSigner) sign(m *v1manifest.RawManifest) (sr *simpleResponse, err statusError) {
	root := v1manifest.Root{}
	if err := json.Unmarshal(m.Signed, &root); err != nil {
		logprinter.Errorf("Unmarshal manifest %s", err.Error())
		return nil, ErrorInvalidManifest
	}

	manifest := &v1manifest.Manifest{
		Signatures: m.Signatures,
		Signed:     &root,
	}

	switch err := h.mirror.Rotate(manifest); err {
	case model.ErrorConflict:
		return nil, ErrorManifestConflict
	case nil:
		return nil, nil
	default:
		logprinter.Errorf("Rotate root manifest: %s", err.Error())
		return nil, ErrorInternalError
	}
}
