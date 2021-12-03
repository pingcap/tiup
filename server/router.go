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

package main

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"
	logprinter "github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/server/handler"
)

type traceResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *traceResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func httpRequestMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logprinter.Infof("Request : %s - %s - %s", r.RemoteAddr, r.Method, r.URL)
		start := time.Now()
		tw := &traceResponseWriter{w, http.StatusOK}
		h.ServeHTTP(tw, r)
		logprinter.Infof("Response [%d] : %s - %s - %s (%.3f sec)",
			tw.statusCode, r.RemoteAddr, r.Method, r.URL, time.Since(start).Seconds())
	})
}

func (s *server) router() http.Handler {
	r := mux.NewRouter()

	r.Handle("/api/v1/tarball/{sid}", handler.UploadTarbal(s.sm))
	r.Handle("/api/v1/component/{sid}/{name}", handler.SignComponent(s.sm, s.mirror))
	r.Handle("/api/v1/rotate", handler.RotateRoot(s.mirror))
	r.PathPrefix("/").Handler(s.static("/", s.mirror.Source(), s.upstream))

	return httpRequestMiddleware(r)
}
