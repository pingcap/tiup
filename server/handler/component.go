package handler

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/pingcap-incubator/tiup/pkg/log"
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

func (h *componentSigner) sign(r *http.Request, m *model.ComponentManifest) (sr *simpleResponse, err statusError) {
	sid := mux.Vars(r)["sid"]
	name := mux.Vars(r)["name"]

	log.Infof("Sign component manifest for %s, sid: %s", name, sid)
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
		fi, err := txn.Stat(fmt.Sprintf("%d.%s.json", m.Signed.Version, name))
		if err != nil {
			return err
		}

		var indexVersion uint
		if err := md.UpdateIndexManifest(func(om *model.IndexManifest) *model.IndexManifest {
			om.Signed.Components[name] = v1manifest.ComponentItem{
				Owner: "pingcap", // TODO: read this from request
				URL:   fmt.Sprintf("/%s.json", name),
			}
			indexVersion = om.Signed.Version + 1
			return om
		}); err != nil {
			return err
		}
		indexFi, err := txn.Stat(fmt.Sprintf("%d.index.json", indexVersion))
		if err != nil {
			return err
		}

		if err := md.UpdateSnapshotManifest(func(om *model.SnapshotManifest) *model.SnapshotManifest {
			om.Signed.Meta["/index.json"] = v1manifest.FileVersion{
				Version: indexVersion,
				Length:  uint(indexFi.Size()),
			}
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
		log.Infof("Sign error: %s", err.Error())
		return err == store.ErrorFsCommitConflict && txn.ResetManifest() == nil
	}); err != nil {
		log.Errorf("Sign component failed: %s", err.Error())
		if err, ok := err.(statusError); ok {
			return nil, err
		}
		return nil, ErrorInternalError
	}

	h.sm.Delete(sid)
	return nil, nil
}
