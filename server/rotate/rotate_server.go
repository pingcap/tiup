package rotate

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"

	cjson "github.com/gibson042/canonicaljson-go"
	"github.com/gorilla/mux"
	"github.com/pingcap/errors"
	"github.com/pingcap/fn"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/pingcap/tiup/pkg/tui/progress"
)

type statusRender struct {
	mbar *progress.MultiBar
	bars map[string]*progress.MultiBarItem
}

func newStatusRender(manifest *v1manifest.Manifest, addr string) *statusRender {
	ss := strings.Split(addr, ":")
	if strings.Trim(ss[0], " ") == "" || strings.Trim(ss[0], " ") == "0.0.0.0" {
		addrs, _ := net.InterfaceAddrs()
		for _, addr := range addrs {
			if ip, ok := addr.(*net.IPNet); ok && !ip.IP.IsLoopback() && ip.IP.To4() != nil {
				ss[0] = ip.IP.To4().String()
				break
			}
		}
	}

	status := &statusRender{
		mbar: progress.NewMultiBar(fmt.Sprintf("Waiting all administrators to sign http://%s/rotate/root.json", strings.Join(ss, ":"))),
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
	r := mux.NewRouter()

	r.Handle("/rotate/root.json", fn.Wrap(func() (*v1manifest.Manifest, error) {
		return &v1manifest.Manifest{Signed: root}, nil
	})).Methods("GET")

	sigCh := make(chan v1manifest.Signature)
	r.Handle("/rotate/root.json", fn.Wrap(func(m *v1manifest.RawManifest) (*v1manifest.Manifest /* always nil */, error) {
		for _, sig := range m.Signatures {
			if err := verifySig(sig, root); err != nil {
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

func verifySig(sig v1manifest.Signature, root *v1manifest.Root) error {
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
