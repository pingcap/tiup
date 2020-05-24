package main

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/pingcap-incubator/tiup/server/handler"
)

func (s *server) router() http.Handler {
	r := mux.NewRouter()

	r.Handle("/api/v1/tarbal/{sid}", handler.UploadTarbal(s.sm))
	r.Handle("/api/v1/component/{sid}/{name}", handler.SignComponent(s.sm, s.keys))
	r.PathPrefix("/").Handler(s.static("/", s.root))

	return r
}
