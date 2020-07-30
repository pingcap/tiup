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
	"fmt"
	"net/http"
	"time"

	cjson "github.com/gibson042/canonicaljson-go"
	"github.com/gorilla/mux"
	"github.com/pingcap/fn"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/pingcap/tiup/server/model"
	"github.com/pingcap/tiup/server/session"
	"github.com/pingcap/tiup/server/store"
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
	options := make(map[string]bool)

	for _, opt := range []string{"yanked", "standalone", "hidden"} {
		if query(r, opt) == "true" {
			options[opt] = true
		} else if query(r, opt) == "false" {
			options[opt] = false
		}
	}

	blackList := []string{"root", "index", "snapshot", "timestamp"}
	for _, b := range blackList {
		if name == b {
			return nil, ErrorForbiden
		}
	}

	log.Infof("Sign component manifest for %s, sid: %s", name, sid)
	txn := h.sm.Load(sid)
	if txn == nil {
		if e := h.sm.Begin(sid); e != nil {
			log.Errorf("Begin session %s", e.Error())
			return nil, ErrorInternalError
		}
		if txn = h.sm.Load(sid); txn == nil {
			return nil, ErrorSessionMissing
		}
	}

	initTime := time.Now()

	md := model.New(txn, h.keys)
	// retry util there is no conflict with other txns
	if err := utils.RetryUntil(func() error {
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

		var indexFileVersion *v1manifest.FileVersion
		var owner *v1manifest.Owner
		if err := md.UpdateIndexManifest(initTime, func(om *model.IndexManifest) *model.IndexManifest {
			// We only update index.json when it's a new component
			// or the yanked, standalone, hidden fileds changed
			var (
				compItem  v1manifest.ComponentItem
				compExist bool
			)

			if compItem, compExist = om.Signed.Components[name]; compExist {
				// Find the owner of target component
				o := om.Signed.Owners[compItem.Owner]
				owner = &o
				if len(options) == 0 {
					// No changes on index.json
					return nil
				}
				if opt, ok := options["yanked"]; ok {
					compItem.Yanked = opt
				}
				if opt, ok := options["hidden"]; ok {
					compItem.Hidden = opt
				}
				if opt, ok := options["standalone"]; ok {
					compItem.Standalone = opt
				}
			} else {
				var ownerID string
				// The component is a new component, so the owner is whoever first create it.
				for _, sk := range m.Signatures {
					if ownerID, owner = om.KeyOwner(sk.KeyID); owner != nil {
						break
					}
				}
				compItem = v1manifest.ComponentItem{
					Owner:      ownerID,
					URL:        fmt.Sprintf("/%s.json", name),
					Yanked:     options["yanked"],
					Standalone: options["standalone"],
					Hidden:     options["hidden"],
				}
			}

			om.Signed.Components[name] = compItem
			indexFileVersion = &v1manifest.FileVersion{Version: om.Signed.Version + 1}
			return om
		}); err != nil {
			return err
		}

		if err := validate(owner, m); err != nil {
			return err
		}

		if indexFileVersion != nil {
			if indexFi, err := txn.Stat(fmt.Sprintf("%d.index.json", indexFileVersion.Version)); err == nil {
				indexFileVersion.Length = uint(indexFi.Size())
			} else {
				return err
			}
		}

		if err := md.UpdateSnapshotManifest(initTime, func(om *model.SnapshotManifest) *model.SnapshotManifest {
			if indexFileVersion != nil {
				om.Signed.Meta["/index.json"] = *indexFileVersion
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
		if err := md.UpdateTimestampManifest(initTime); err != nil {
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

// ModifyComponent handles requests to modify index.json (yank or hide components)
func ModifyComponent(sm session.Manager, keys map[string]*v1manifest.KeyInfo) http.Handler {
	return &componentSigner{sm, keys}
}

func validate(owner *v1manifest.Owner, m *model.ComponentManifest) error {
	if owner == nil {
		return ErrorForbiden
	}

	payload, err := cjson.Marshal(m.Signed)
	if err != nil {
		return err
	}

	for _, s := range m.Signatures {
		k := owner.Keys[s.KeyID]
		if k == nil {
			continue
		}

		if err := k.Verify(payload, s.Sig); err == nil {
			return nil
		}
	}

	return ErrorForbiden
}

func query(r *http.Request, q string) string {
	qs := r.URL.Query()[q]

	if len(qs) == 0 {
		return ""
	}

	return qs[0]
}
