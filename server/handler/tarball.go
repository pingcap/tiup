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
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/server/session"
)

// MaxFileSize is the max size content can be uploaded
const MaxFileSize = 32 * 1024 * 1024

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
	log.Infof("Uploading tarball, sid: %s", sid)

	if err := h.sm.Begin(sid); err != nil {
		if err == session.ErrorSessionConflict {
			log.Warnf("Session already exists, this is a retransmission, try to restart session")
			// Reset manifest to avoid conflict
			if err := h.sm.Load(sid).ResetManifest(); err != nil {
				log.Errorf("Failed to restart session: %s", err.Error())
				return nil, ErrorInternalError
			}
			log.Infof("Restart session success")
		} else {
			log.Errorf("Failed to start session: %s", err.Error())
			return nil, ErrorInternalError
		}
	}

	txn := h.sm.Load(sid)

	if err := r.ParseMultipartForm(MaxFileSize); err != nil {
		// TODO: log error here
		return nil, ErrorInvalidTarball
	}

	file, handler, err := r.FormFile("file")
	if err != nil {
		// TODO: log error here
		return nil, ErrorInvalidTarball
	}
	defer file.Close()

	if err := txn.Write(handler.Filename, file); err != nil {
		log.Errorf("Error to write tarball: %s", err.Error())
		return nil, ErrorInternalError
	}

	return nil, nil
}
