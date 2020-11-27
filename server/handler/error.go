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

import "net/http"

type statusError interface {
	StatusCode() int
	Status() string
	Error() string
}

type handlerError struct {
	code    int
	status  string
	message string
}

func newHandlerError(code int, status, message string) statusError {
	return &handlerError{
		code:    code,
		status:  status,
		message: message,
	}
}

func (e *handlerError) StatusCode() int {
	return e.code
}

func (e *handlerError) Status() string {
	return e.status
}

func (e *handlerError) Error() string {
	return e.message
}

var (
	// ErrorSessionMissing indicates that the specified session not found
	ErrorSessionMissing = newHandlerError(http.StatusNotFound, "SESSION NOT FOUND", "session with specified identity not found")
	// ErrorManifestMissing indicates that the specified component doesn't have manifest yet
	ErrorManifestMissing = newHandlerError(http.StatusNotFound, "MANIFEST NOT FOUND", "that component doesn't have manifest yet")
	// ErrorInvalidTarball indicates that the tarball is not valid (eg. too large)
	ErrorInvalidTarball = newHandlerError(http.StatusBadRequest, "INVALID TARBALL", "the tarball content is not valid")
	// ErrorInvalidManifest indicates that the manifest is not valid
	ErrorInvalidManifest = newHandlerError(http.StatusBadRequest, "INVALID MANIFEST", "the manifest content is not valid")
	// ErrorInternalError indicates that an internal error happened
	ErrorInternalError = newHandlerError(http.StatusInternalServerError, "INTERNAL ERROR", "an internal error happened")
	// ErrorManifestConflict indicates that the uploaded manifest is not new enough
	ErrorManifestConflict = newHandlerError(http.StatusConflict, "MANIFEST CONFLICT", "the manifest provided is not new enough")
	// ErrorForbiden indicates that the user can't access target resource
	ErrorForbiden = newHandlerError(http.StatusForbidden, "FORBIDDEN", "permission denied")
)
