package main

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/pingcap-incubator/tiup/pkg/log"
	"github.com/pingcap-incubator/tiup/server/handler"
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
		log.Infof("Request : %s - %s - %s", r.RemoteAddr, r.Method, r.URL)
		start := time.Now()
		tw := &traceResponseWriter{w, http.StatusOK}
		h.ServeHTTP(tw, r)
		log.Infof("Response [%d] : %s - %s - %s (%.3f sec)",
			tw.statusCode, r.RemoteAddr, r.Method, r.URL, time.Since(start).Seconds())
	})
}

func (s *server) router() http.Handler {
	r := mux.NewRouter()

	r.Handle("/api/v1/tarbal/{sid}", handler.UploadTarbal(s.sm))
	r.Handle("/api/v1/component/{sid}/{name}", handler.SignComponent(s.sm, s.keys))
	r.PathPrefix("/").Handler(s.static("/", s.root))

	return httpRequestMiddleware(r)
}
