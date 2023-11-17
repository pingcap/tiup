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

package task

import (
	"context"
	"encoding/pem"
	"fmt"
	"net"
	"path/filepath"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/crypto"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/utils"
)

// TLSCert generates a certificate for instance
type TLSCert struct {
	comp  string
	role  string
	host  string
	port  int
	ca    *crypto.CertificateAuthority
	paths meta.DirPaths
}

// Execute implements the Task interface
func (c *TLSCert) Execute(ctx context.Context) error {
	privKey, err := crypto.NewKeyPair(crypto.KeyTypeRSA, crypto.KeySchemeRSASSAPSSSHA256)
	if err != nil {
		return err
	}

	// Add localhost and 127.0.0.1 to the trust list,
	// then it is easy for some scripts to request a local interface directly
	hosts := []string{"localhost"}
	ips := []string{"127.0.0.1"}
	if host := c.host; net.ParseIP(host) != nil && host != "127.0.0.1" {
		ips = append(ips, host)
	} else if host != "localhost" {
		hosts = append(hosts, host)
	}
	csr, err := privKey.CSR(c.role, c.comp, hosts, ips)
	if err != nil {
		return err
	}
	cert, err := c.ca.Sign(csr)
	if err != nil {
		return err
	}

	// make sure the cache dir exist
	if err := utils.MkdirAll(c.paths.Cache, 0755); err != nil {
		return err
	}

	// save cert to cache dir
	keyFileName := fmt.Sprintf("%s-%s-%d.pem", c.role, c.host, c.port)
	certFileName := fmt.Sprintf("%s-%s-%d.crt", c.role, c.host, c.port)
	keyFile := filepath.Join(
		c.paths.Cache,
		keyFileName,
	)
	certFile := filepath.Join(
		c.paths.Cache,
		certFileName,
	)
	caFile := filepath.Join(c.paths.Cache, spec.TLSCACert)
	if err := utils.SaveFileWithBackup(keyFile, privKey.Pem(), ""); err != nil {
		return err
	}
	if err := utils.SaveFileWithBackup(certFile, pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert,
	}), ""); err != nil {
		return err
	}
	if err := utils.SaveFileWithBackup(caFile, pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: c.ca.Cert.Raw,
	}), ""); err != nil {
		return err
	}

	// transfer file to remote
	e, ok := ctxt.GetInner(ctx).GetExecutor(c.host)
	if !ok {
		return ErrNoExecutor
	}
	if err := e.Transfer(ctx, caFile,
		filepath.Join(c.paths.Deploy, spec.TLSCertKeyDir, spec.TLSCACert),
		false, /* download */
		0,     /* limit */
		false /* compress */); err != nil {
		return errors.Annotate(err, "failed to transfer CA cert to server")
	}
	if err := e.Transfer(ctx, keyFile,
		filepath.Join(c.paths.Deploy, spec.TLSCertKeyDir, fmt.Sprintf("%s.pem", c.role)),
		false, /* download */
		0,     /* limit */
		false /* compress */); err != nil {
		return errors.Annotate(err, "failed to transfer TLS private key to server")
	}
	if err := e.Transfer(ctx, certFile,
		filepath.Join(c.paths.Deploy, spec.TLSCertKeyDir, fmt.Sprintf("%s.crt", c.role)),
		false, /* download */
		0,     /* limit */
		false /* compress */); err != nil {
		return errors.Annotate(err, "failed to transfer TLS cert to server")
	}

	return nil
}

// Rollback implements the Task interface
func (c *TLSCert) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *TLSCert) String() string {
	return fmt.Sprintf("TLSCert: host=%s role=%s cn=%s", c.host, c.role, c.comp)
}
