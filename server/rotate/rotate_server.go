package rotate

import (
	"context"
	"fmt"
	"net/http"

	cjson "github.com/gibson042/canonicaljson-go"
	"github.com/gorilla/mux"
	"github.com/pingcap/errors"
	"github.com/pingcap/fn"
	"github.com/pingcap/tiup/pkg/cliutil/progress"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
)

type statusRender struct {
	mbar *progress.MultiBar
	bars map[string]*progress.MultiBarItem
}

func newStatusRender(manifest *v1manifest.Manifest) *statusRender {
	status := &statusRender{
		mbar: progress.NewMultiBar("Waiting all administrators to sign"),
		bars: make(map[string]*progress.MultiBarItem),
	}
	root := manifest.Signed.(*v1manifest.Root)
	for key := range root.Roles[v1manifest.ManifestTypeRoot].Keys {
		status.bars[key] = status.mbar.AddBar(fmt.Sprintf("  - Waiting key %s", key))
	}
	status.mbar.StartRenderLoop()
	return status
}

func (s *statusRender) render(manifest *v1manifest.Manifest) {
	for _, sig := range manifest.Signatures {
		s.bars[sig.KeyID].UpdateDisplay(&progress.DisplayProps{
			Prefix: fmt.Sprintf("  - Waiting key %s", sig.KeyID),
			Mode:   progress.ModeDone,
		})
	}
}

func (s *statusRender) stop() {
	s.mbar.StopRenderLoop()
}

// Serve starts a temp server for receiving signatures from administrators
func Serve(addr string, root *v1manifest.Root) (*v1manifest.Manifest, error) {
	payload, err := cjson.Marshal(root)
	if err != nil {
		return nil, fn.ErrorWithStatusCode(errors.Annotate(err, "marshal root manifest"), http.StatusInternalServerError)
	}

	r := mux.NewRouter()

	r.Handle("/rotate/root.json", fn.Wrap(func() (*v1manifest.Manifest, error) {
		return &v1manifest.Manifest{Signed: root}, nil
	})).Methods("GET")

	sigCh := make(chan v1manifest.Signature)
	r.Handle("/rotate/root.json", fn.Wrap(func(m *v1manifest.RawManifest) (*v1manifest.Manifest /* always nil */, error) {
		for _, sig := range m.Signatures {
			sigCh <- sig
		}
		return nil, nil
	})).Methods("POST")

	srv := &http.Server{Addr: addr, Handler: r}
	go func() {
		if err = srv.ListenAndServe(); err != http.ErrServerClosed {
			close(sigCh)
		}
	}()

	manifest := &v1manifest.Manifest{Signed: root}
	status := newStatusRender(manifest)
	defer status.stop()
	for sig := range sigCh {
		k := root.Roles[v1manifest.ManifestTypeRoot].Keys[sig.KeyID]
		if k == nil {
			// Received a signature signed by an invalid key
			continue
		}
		if err := k.Verify(payload, sig.Sig); err != nil {
			// Received an invalid signature
			continue
		}
		for _, s := range manifest.Signatures {
			if s.KeyID == sig.KeyID {
				// Duplicate signature
				continue
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
		return nil, err
	}
	return manifest, nil
}
