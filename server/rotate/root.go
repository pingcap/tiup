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

package rotate

import (
	"context"
	"fmt"
	"net/http"

	cjson "github.com/gibson042/canonicaljson-go"
	"github.com/gorilla/mux"
	"github.com/pingcap/errors"
	"github.com/pingcap/fn"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/pingcap/tiup/pkg/utils"
)

// ServeRoot starts a temp server for receiving root signatures from administrators
func ServeRoot(addr string, root *v1manifest.Root) (*v1manifest.Manifest, error) {
	r := mux.NewRouter()
	uri := fmt.Sprintf("/rotate/%s", utils.Base62Tag())

	r.Handle(uri, fn.Wrap(func() (*v1manifest.Manifest, error) {
		return &v1manifest.Manifest{Signed: root}, nil
	})).Methods("GET")

	sigCh := make(chan v1manifest.Signature)
	r.Handle(uri, fn.Wrap(func(m *v1manifest.RawManifest) (*v1manifest.Manifest /* always nil */, error) {
		for _, sig := range m.Signatures {
			if err := verifyRootSig(sig, root); err != nil {
				return nil, err
			}
			sigCh <- sig
		}
		return nil, nil
	})).Methods("POST")

	srv := &http.Server{Addr: addr, Handler: r}
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			logprinter.Errorf("server closed: %s", err.Error())
		}
		close(sigCh)
	}()

	manifest := &v1manifest.Manifest{Signed: root}
	status := newStatusRender(manifest, addr)
	defer status.stop()

SIGLOOP:
	for sig := range sigCh {
		for _, s := range manifest.Signatures {
			if s.KeyID == sig.KeyID {
				// Duplicate signature
				continue SIGLOOP
			}
		}
		manifest.Signatures = append(manifest.Signatures, sig)
		status.render(manifest)
		if len(manifest.Signatures) == len(root.Roles[v1manifest.ManifestTypeRoot].Keys) {
			_ = srv.Shutdown(context.Background())
			break
		}
	}

	if len(manifest.Signatures) != len(root.Roles[v1manifest.ManifestTypeRoot].Keys) {
		return nil, errors.New("no enough signature collected before server shutdown")
	}
	return manifest, nil
}

func verifyRootSig(sig v1manifest.Signature, root *v1manifest.Root) error {
	payload, err := cjson.Marshal(root)
	if err != nil {
		return fn.ErrorWithStatusCode(errors.Annotate(err, "marshal root manifest"), http.StatusInternalServerError)
	}

	k := root.Roles[v1manifest.ManifestTypeRoot].Keys[sig.KeyID]
	if k == nil {
		// Received a signature signed by an invalid key
		return fn.ErrorWithStatusCode(errors.New("the key is not valid"), http.StatusNotAcceptable)
	}
	if err := k.Verify(payload, sig.Sig); err != nil {
		// Received an invalid signature
		return fn.ErrorWithStatusCode(errors.New("the signature is not valid"), http.StatusNotAcceptable)
	}
	return nil
}
