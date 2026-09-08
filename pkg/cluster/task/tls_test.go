// Copyright 2026 PingCAP, Inc.
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

package task

import (
	"context"
	"encoding/pem"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/crypto"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/stretchr/testify/require"
)

type overlappingCATransfer struct {
	firstCA     string
	firstReady  chan struct{}
	readFirst   chan struct{}
	firstRead   chan struct{}
	mu          sync.Mutex
	transferred map[string][]byte
}

func (*overlappingCATransfer) Execute(context.Context, string, bool, ...time.Duration) ([]byte, []byte, error) {
	return nil, nil, nil
}

func (e *overlappingCATransfer) Transfer(ctx context.Context, src, dst string, _ bool, _ int, _ bool) error {
	if filepath.Base(dst) != spec.TLSCACert {
		return nil
	}
	if dst == e.firstCA {
		close(e.firstReady)
		select {
		case <-e.readFirst:
		case <-ctx.Done():
			return ctx.Err()
		}
		data, err := os.ReadFile(src)
		e.mu.Lock()
		e.transferred[dst] = data
		e.mu.Unlock()
		close(e.firstRead)
		return err
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	// Reproduce a competing writer's truncation window without relying on
	// scheduler timing. It must not corrupt the first task's in-flight source.
	if err := os.Truncate(src, 0); err != nil {
		return err
	}
	close(e.readFirst)
	select {
	case <-e.firstRead:
	case <-ctx.Done():
		return ctx.Err()
	}
	if err := os.WriteFile(src, data, 0600); err != nil {
		return err
	}
	e.mu.Lock()
	e.transferred[dst] = data
	e.mu.Unlock()
	return nil
}

func TestTLSCertPreservesInFlightCA(t *testing.T) {
	ca, err := crypto.NewCA("test")
	require.NoError(t, err)
	expected := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.Cert.Raw})
	for _, tc := range []struct {
		name string
		host string
		port int
	}{
		{name: "different_hosts", host: "n2", port: 20160},
		{name: "same_host_different_ports", host: "n1", port: 20161},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			first := &TLSCert{comp: "tikv", role: "tikv", host: "n1", port: 20160, ca: ca,
				paths: meta.DirPaths{Deploy: filepath.Join(root, "first"), Cache: filepath.Join(root, "cache")}}
			second := &TLSCert{comp: "tikv", role: "tikv", host: tc.host, port: tc.port, ca: ca,
				paths: meta.DirPaths{Deploy: filepath.Join(root, "second"), Cache: first.paths.Cache}}
			firstCA := filepath.Join(first.paths.Deploy, spec.TLSCertKeyDir, spec.TLSCACert)
			secondCA := filepath.Join(second.paths.Deploy, spec.TLSCertKeyDir, spec.TLSCACert)
			e := &overlappingCATransfer{firstCA: firstCA, firstReady: make(chan struct{}),
				readFirst: make(chan struct{}), firstRead: make(chan struct{}), transferred: make(map[string][]byte)}
			base, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			ctx := ctxt.New(base, 2, logprinter.NewLogger(""))
			ctxt.GetInner(ctx).SetExecutor(first.host, e)
			ctxt.GetInner(ctx).SetExecutor(second.host, e)
			done := make(chan error, 1)
			go func() { done <- first.Execute(ctx) }()
			select {
			case <-e.firstReady:
			case err := <-done:
				require.NoError(t, err)
				t.Fatal("first task did not reach CA transfer")
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			require.NoError(t, second.Execute(ctx))
			require.NoError(t, <-done)
			e.mu.Lock()
			defer e.mu.Unlock()
			require.Equal(t, expected, e.transferred[firstCA])
			require.Equal(t, expected, e.transferred[secondCA])
		})
	}
}
