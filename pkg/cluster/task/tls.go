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
	"encoding/pem"
	"fmt"
	"net"
	"path/filepath"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/crypto"
	"github.com/pingcap/tiup/pkg/file"
	"github.com/pingcap/tiup/pkg/meta"
)

// TLSCert generates a certificate for instance
type TLSCert struct {
	inst  spec.Instance
	ca    *crypto.CertificateAuthority
	paths meta.DirPaths
}

// Execute implements the Task interface
func (c *TLSCert) Execute(ctx *Context) error {
	privKey, err := crypto.NewKeyPair(crypto.KeyTypeRSA, crypto.KeySchemeRSASSAPSSSHA256)
	if err != nil {
		return err
	}

	hosts := []string{c.inst.GetHost()}
	ips := []string{}
	if net.ParseIP(c.inst.GetHost()) != nil {
		hosts, ips = ips, hosts
	}
	csr, err := privKey.CSR(c.inst.Role(), c.inst.ComponentName(), hosts, ips)
	if err != nil {
		return err
	}
	cert, err := c.ca.Sign(csr)
	if err != nil {
		return err
	}

	// save cert to cache dir
	keyFileName := fmt.Sprintf("%s-%s-%d.pem", c.inst.Role(), c.inst.GetHost(), c.inst.GetMainPort())
	certFileName := fmt.Sprintf("%s-%s-%d.crt", c.inst.Role(), c.inst.GetHost(), c.inst.GetMainPort())
	keyFile := filepath.Join(
		c.paths.Cache,
		keyFileName,
	)
	certFile := filepath.Join(
		c.paths.Cache,
		certFileName,
	)
	caFile := filepath.Join(c.paths.Cache, spec.TLSCACert)
	if err := file.SaveFileWithBackup(keyFile, privKey.Pem(), ""); err != nil {
		return err
	}
	if err := file.SaveFileWithBackup(certFile, pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert,
	}), ""); err != nil {
		return err
	}
	if err := file.SaveFileWithBackup(caFile, pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: c.ca.Cert.Raw,
	}), ""); err != nil {
		return err
	}

	// transfer file to remote
	e, ok := ctx.GetExecutor(c.inst.GetHost())
	if !ok {
		return ErrNoExecutor
	}
	if err := e.Transfer(caFile,
		filepath.Join(c.paths.Deploy, "tls", spec.TLSCACert),
		false /* download */); err != nil {
		return errors.Annotate(err, "failed to transfer CA cert to server")
	}
	if err := e.Transfer(keyFile,
		filepath.Join(c.paths.Deploy, "tls", fmt.Sprintf("%s.pem", c.inst.Role())),
		false /* download */); err != nil {
		return errors.Annotate(err, "failed to transfer TLS private key to server")
	}
	if err := e.Transfer(certFile,
		filepath.Join(c.paths.Deploy, "tls", fmt.Sprintf("%s.crt", c.inst.Role())),
		false /* download */); err != nil {
		return errors.Annotate(err, "failed to transfer TLS cert to server")
	}

	return nil
}

// Rollback implements the Task interface
func (c *TLSCert) Rollback(ctx *Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *TLSCert) String() string {
	return fmt.Sprintf("TLSCert: host=%s role=%s cn=%s",
		c.inst.GetHost(), c.inst.Role(), c.inst.ComponentName())
}
