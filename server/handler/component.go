package handler

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/pingcap-incubator/tiup/pkg/repository/v1manifest"
	"github.com/pingcap-incubator/tiup/pkg/utils"
	"github.com/pingcap-incubator/tiup/server/model"
	"github.com/pingcap-incubator/tiup/server/session"
	"github.com/pingcap-incubator/tiup/server/store"
	"github.com/pingcap/fn"
)

// SignComponent handles requests to re-sign component manifest
func SignComponent(sm session.Manager, keys map[string]*v1manifest.KeyInfo) http.Handler {
	return &componentSigner{sm, keys}
}

type componentSigner struct {
	sm   session.Manager
	keys map[string]*v1manifest.KeyInfo
}

func (h *componentSigner) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fn.Wrap(h.sign).ServeHTTP(w, r)
}

func (h *componentSigner) sign(r *http.Request, m *model.ComponentManifest) (sr *simpleResponse, err error) {
	sid := mux.Vars(r)["sid"]
	name := mux.Vars(r)["name"]

	txn := h.sm.Load(sid)
	if txn == nil {
		return nil, ErrorSessionMissing
	}

	md := model.New(txn, h.keys)
	// Retry util not conflict with other txns
	if err := utils.Retry(func() error {
		// Write the component manifest (component.json)
		if err := md.UpdateComponentManifest(name, m); err != nil {
			if err == model.ErrorConflict {
				return ErrorManifestConflict
			}
			return err
		}

		// Update snapshot.json and signature
		fi, err := txn.Stat(fmt.Sprintf("%s.json", name))
		if err != nil {
			return err
		}

		if err := md.UpdateSnapshotManifest(func(om *model.SnapshotManifest) *model.SnapshotManifest {
			om.Signed.Meta[fmt.Sprintf("/%s.json", name)] = v1manifest.FileVersion{
				Version: m.Signed.Version,
				Length:  uint(fi.Size()),
			}
			return om
		}); err != nil {
			return err
		}

		// Update timestamp.json and signature
		if err := md.UpdateTimestampManifest(); err != nil {
			return err
		}
		return txn.Commit()
	}, func(err error) bool {
		return err == store.ErrorFsCommitConflict && txn.ResetManifest() == nil
	}); err != nil {
		return nil, ErrorInternalError
	}

	h.sm.Delete(sid)
	return nil, nil
}
