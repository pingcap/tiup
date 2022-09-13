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
	"net/http"

	"github.com/gorilla/mux"
	"github.com/pingcap/fn"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/server/session"
)

// MaxMemory is the a total of max bytes of its file parts stored in memory
const MaxMemory = 32 * 1024 * 1024

// UploadTarbal handle tarball upload
func UploadTarbal(sm session.Manager) http.Handler {
	return &tarballUploader{sm}
}

type tarballUploader struct {
	sm session.Manager
}

func (h *tarballUploader) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fn.Wrap(h.upload).ServeHTTP(w, r)
}

func (h *tarballUploader) upload(r *http.Request) (*simpleResponse, statusError) {
	sid := mux.Vars(r)["sid"]
	logprinter.Infof("Uploading tarball, sid: %s", sid)

	file, handler, err := r.FormFile("file")
	if err != nil {
		// TODO: log error here
		return nil, ErrorInvalidTarball
	}
	defer file.Close()
	defer r.MultipartForm.RemoveAll()

	if err := h.sm.Write(sid, handler.Filename, file); err != nil {
		logprinter.Errorf("Error to write tarball: %s", err.Error())
		return nil, ErrorInternalError
	}

	return nil, nil
}
