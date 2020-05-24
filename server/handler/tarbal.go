package handler

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/pingcap-incubator/tiup/server/session"
	"github.com/pingcap/fn"
)

// MaxFileSize is the max size content can be uploaded
const MaxFileSize = 32 * 1024 * 1024

// UploadTarbal handle tarbal upload
func UploadTarbal(sm session.Manager) http.Handler {
	return &tarbalUploader{sm}
}

type tarbalUploader struct {
	sm session.Manager
}

func (h *tarbalUploader) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fn.Wrap(h.upload).ServeHTTP(w, r)
}

func (h *tarbalUploader) upload(r *http.Request) (*simpleResponse, error) {
	sid := mux.Vars(r)["sid"]
	if err := h.sm.Begin(sid); err != nil {
		return nil, ErrorTarbalConflict
	}

	txn := h.sm.Load(sid)

	if err := r.ParseMultipartForm(MaxFileSize); err != nil {
		// TODO: log error here
		return nil, ErrorInvalidTarbal
	}

	file, handler, err := r.FormFile("file")
	if err != nil {
		// TODO: log error here
		return nil, ErrorInvalidTarbal
	}
	defer file.Close()

	if err := txn.Write(handler.Filename, file); err != nil {
		// TODO: log error here
		return nil, ErrorInternalError
	}

	return nil, nil
}
